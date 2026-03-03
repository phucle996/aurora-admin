package app

import (
	etcdinfra "admin/infra/etcd"
	telegraminfra "admin/infra/telegram"
	"admin/internal/config"
	apisvc "admin/internal/service"
	"admin/pkg/logger"
	"context"
	"errors"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Modules struct {
	// Infrastructure
	Etcd *clientv3.Client

	APIKeySvc      *apisvc.APIKeyService
	TokenSecretSvc *apisvc.TokenSecretService
	CertStoreSvc   *apisvc.CertStoreService
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

	telegramClient, err := telegraminfra.NewClient(&cfg.Telegram)
	if err != nil {
		_ = etcdClient.Close()
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

	apiKeySvc := apisvc.NewAPIKeyService(etcdClient, apisvc.APIKeyServiceConfig{
		Prefix:         cfg.APIKey.Prefix,
		RotateInterval: cfg.APIKey.RotateInterval,
		OnBootstrap:    onBootstrap,
		OnRotate:       onRotate,
	})
	if _, err := apiKeySvc.BootstrapAPIKey(ctx); err != nil {
		_ = etcdClient.Close()
		return nil, err
	}

	tokenSecretSvc := apisvc.NewTokenSecretService(etcdClient, apisvc.TokenSecretServiceConfig{
		Prefix:                cfg.TokenSecret.Prefix,
		AccessRotateInterval:  cfg.TokenSecret.AccessRotateInterval,
		RefreshRotateInterval: cfg.TokenSecret.RefreshRotateInterval,
		DeviceRotateInterval:  cfg.TokenSecret.DeviceRotateInterval,
	})
	if err := tokenSecretSvc.Bootstrap(ctx); err != nil {
		_ = etcdClient.Close()
		return nil, err
	}
	if err := tokenSecretSvc.RotateDueSecrets(ctx); err != nil {
		_ = etcdClient.Close()
		return nil, err
	}

	certStoreSvc := apisvc.NewCertStoreService(etcdClient, apisvc.CertStoreServiceConfig{
		Prefix: cfg.CertStore.Prefix,
	})

	return &Modules{
		Etcd:           etcdClient,
		APIKeySvc:      apiKeySvc,
		TokenSecretSvc: tokenSecretSvc,
		CertStoreSvc:   certStoreSvc,
	}, nil
}
