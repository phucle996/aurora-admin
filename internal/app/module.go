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
		cfg.ModuleInstall.UMSInstallScriptURL,
		cfg.ModuleInstall.PlatformInstallScriptURL,
		cfg.ModuleInstall.PaaSInstallScriptURL,
		cfg.ModuleInstall.DBaaSInstallScriptURL,
		cfg.ModuleInstall.UIInstallScriptURL,
	)
	runtimeSvc := apisvc.NewRuntimeBootstrapService(runtimeRepo, enabledModuleRepo, certStoreRepo, cfg.CertStore.Prefix)

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
