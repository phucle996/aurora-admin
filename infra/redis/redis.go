package redisinfra

import (
	"admin/internal/config"
	"admin/pkg/logger"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

func NewRedis(ctx context.Context, cfg *config.RedisCfg) (*redis.Client, error) {
	const op = "system.redis.connect"
	if cfg == nil {
		return nil, fmt.Errorf("redis config is nil")
	}

	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		logger.SysError(op, err, "build redis tls config failed")
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Addr:      cfg.Addr,
		Username:  cfg.Username,
		Password:  cfg.Password,
		DB:        cfg.DB,
		TLSConfig: tlsCfg,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		logger.SysError(op, err, "redis ping failed")
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	logger.SysInfo(op, "connected redis=%s tls=%t", cfg.Addr, tlsCfg != nil)
	return client, nil
}

func buildTLSConfig(cfg *config.RedisCfg) (*tls.Config, error) {
	if !cfg.UseTLS && cfg.CA == "" && cfg.ClientCert == "" && cfg.ClientKey == "" {
		return nil, nil
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec
	}

	if cfg.CA != "" {
		caPEM, err := os.ReadFile(cfg.CA)
		if err != nil {
			return nil, fmt.Errorf("read redis ca: %w", err)
		}
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caPEM); !ok {
			return nil, fmt.Errorf("invalid redis ca")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("load redis client cert/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}
