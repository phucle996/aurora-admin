package app

import (
	etcdinfra "admin/infra/etcd"
	redisinfra "admin/infra/redis"
	telegraminfra "admin/infra/telegram"
	"admin/internal/config"
	keycfg "admin/internal/key"
	installsvc "admin/internal/moduleinstall"
	"admin/internal/repository"
	apisvc "admin/internal/service"
	"admin/pkg/logger"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Modules struct {
	// Infrastructure
	Etcd  *clientv3.Client
	Redis *redis.Client

	APIKeySvc        *apisvc.APIKeyService
	TokenSecretSvc   *apisvc.TokenSecretService
	CertStoreSvc     *apisvc.CertStoreService
	EnabledModuleSvc *apisvc.EnabledModuleService
	ModuleInstallSvc *installsvc.ModuleInstallService
	RuntimeSvc       *apisvc.RuntimeBootstrapService
}

// NewModules assembles all infrastructure dependencies.
func NewModules(
	ctx context.Context,
	cfg *config.Config,
) (*Modules, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if err := validateAgentMTLSConfig(cfg); err != nil {
		return nil, err
	}

	etcdClient, err := etcdinfra.NewClient(&cfg.Etcd)
	if err != nil {
		return nil, err
	}
	redisClient, err := redisinfra.NewRedis(ctx, &cfg.Redis)
	if err != nil {
		_ = etcdClient.Close()
		return nil, err
	}

	telegramClient, err := telegraminfra.NewClient(&cfg.Telegram)
	if err != nil {
		_ = etcdClient.Close()
		_ = redisClient.Close()
		return nil, err
	}

	var onBootstrap func(context.Context, apisvc.APIKeyBootstrapResult)
	var onRotate func(context.Context, apisvc.APIKeyRotationResult)
	if telegramClient != nil {
		onBootstrap = func(ctx context.Context, result apisvc.APIKeyBootstrapResult) {
			msg := fmt.Sprintf(
				"[ADMIN API KEY]\nEvent: bootstrap\nVersion: %d\nKey: %s\nAt: %s",
				result.Current.Version,
				result.Current.Key,
				time.Now().In(time.Local).Format(time.RFC3339),
			)
			if err := telegramClient.SendMessage(ctx, msg); err != nil {
				logger.SysWarn("apikey.notify", "telegram bootstrap notification failed: %v", err)
			}
		}
		onRotate = func(ctx context.Context, result apisvc.APIKeyRotationResult) {
			msg := fmt.Sprintf(
				"[ADMIN API KEY]\nEvent: rotate\nOld Version: %d\nOld Key: %s\nNew Version: %d\nNew Key: %s\nAt: %s",
				result.Old.Version,
				result.Old.Key,
				result.New.Version,
				result.New.Key,
				time.Now().In(time.Local).Format(time.RFC3339),
			)
			if err := telegramClient.SendMessage(ctx, msg); err != nil {
				logger.SysWarn("apikey.notify", "telegram rotate notification failed: %v", err)
			}
		}
	}

	apiKeySvc := apisvc.NewAPIKeyService(etcdClient, cfg.APIKey, onBootstrap, onRotate)
	if _, err := apiKeySvc.BootstrapAPIKey(ctx); err != nil {
		_ = etcdClient.Close()
		_ = redisClient.Close()
		return nil, err
	}

	tokenSecretCacheRepo := repository.NewRedisTokenSecretCacheRepository(redisClient)
	tokenSecretSvc := apisvc.NewTokenSecretService(etcdClient, cfg.TokenSecret, tokenSecretCacheRepo)
	if err := tokenSecretSvc.Bootstrap(ctx); err != nil {
		_ = etcdClient.Close()
		_ = redisClient.Close()
		return nil, err
	}
	if err := tokenSecretSvc.RotateDueSecrets(ctx); err != nil {
		_ = etcdClient.Close()
		_ = redisClient.Close()
		return nil, err
	}

	certStoreRepo := repository.NewEtcdCertStoreRepository(etcdClient)
	certStoreSvc := apisvc.NewCertStoreService(certStoreRepo, apisvc.CertStoreServiceConfig{
		Prefix: cfg.CertStore.Prefix,
	})

	enabledModuleRepo := repository.NewEtcdEndpointRepository(etcdClient, keycfg.EndpointPrefix)
	runtimeRepo := repository.NewEtcdRuntimeConfigRepository(etcdClient, keycfg.RuntimePrefix)
	enabledModuleSvc := apisvc.NewEnabledModuleService(enabledModuleRepo)
	moduleInstallSvc := installsvc.NewModuleInstallService(
		enabledModuleRepo,
		runtimeRepo,
		certStoreRepo,
		cfg.CertStore.Prefix,
		cfg.Database.URL,
		cfg.AgentMTLS.CACert,
		cfg.AgentMTLS.AdminClientCert,
		cfg.AgentMTLS.AdminClientKey,
		map[string]string{
			"ums":      cfg.ModuleInstall.UMSInstallScriptURL,
			"platform": cfg.ModuleInstall.PlatformInstallScriptURL,
			"paas":     cfg.ModuleInstall.PaaSInstallScriptURL,
			"dbaas":    cfg.ModuleInstall.DBaaSInstallScriptURL,
			"ui":       cfg.ModuleInstall.UIInstallScriptURL,
		},
	)
	runtimeSvc := apisvc.NewRuntimeBootstrapService(
		runtimeRepo,
		enabledModuleRepo,
		certStoreRepo,
		cfg.CertStore.Prefix,
		cfg.App.TLSCA,
		cfg.AgentMTLS.CACert,
		cfg.AgentMTLS.CAKey,
	)
	if err := runtimeSvc.SeedControlPlaneTrustStore(ctx, apisvc.ControlPlaneTrustSeedInput{
		AdminCACertPath:           cfg.App.TLSCA,
		AdminServerCertPath:       cfg.App.TLSCert,
		AgentCACertPath:           cfg.AgentMTLS.CACert,
		AgentSharedClientCertPath: cfg.AgentMTLS.AdminClientCert,
	}); err != nil {
		_ = etcdClient.Close()
		_ = redisClient.Close()
		return nil, err
	}

	return &Modules{
		Etcd:             etcdClient,
		Redis:            redisClient,
		APIKeySvc:        apiKeySvc,
		TokenSecretSvc:   tokenSecretSvc,
		CertStoreSvc:     certStoreSvc,
		EnabledModuleSvc: enabledModuleSvc,
		ModuleInstallSvc: moduleInstallSvc,
		RuntimeSvc:       runtimeSvc,
	}, nil
}

func validateAgentMTLSConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	agentCACert := normalizePath(cfg.AgentMTLS.CACert)
	agentCAKey := normalizePath(cfg.AgentMTLS.CAKey)
	agentClientCert := normalizePath(cfg.AgentMTLS.AdminClientCert)
	agentClientKey := normalizePath(cfg.AgentMTLS.AdminClientKey)
	if agentCACert == "" || agentCAKey == "" || agentClientCert == "" || agentClientKey == "" {
		return fmt.Errorf("agent mTLS config is incomplete (require APP_AGENT_TLS_CA_CERT_FILE, APP_AGENT_TLS_CA_KEY_FILE, APP_AGENT_TLS_ADMIN_CLIENT_CERT_FILE, APP_AGENT_TLS_ADMIN_CLIENT_KEY_FILE)")
	}

	appCACert := normalizePath(cfg.App.TLSCA)
	appCAKey := normalizePath(cfg.App.TLSCAKey)
	if appCACert != "" && appCACert == agentCACert {
		return fmt.Errorf("agent mTLS CA cert must be separate from app TLS CA cert")
	}
	if appCAKey != "" && appCAKey == agentCAKey {
		return fmt.Errorf("agent mTLS CA key must be separate from app TLS CA key")
	}
	return nil
}

func normalizePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	clean := filepath.Clean(trimmed)
	return strings.TrimSpace(clean)
}
