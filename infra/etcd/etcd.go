package etcd

import (
	"admin/internal/config"
	"admin/pkg/logger"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewClient(cfg *config.EtcdCfg) (*clientv3.Client, error) {
	if cfg == nil {
		err := errors.New("etcd config is nil")
		logger.SysError("etcd.init", err, "initialize etcd client failed")
		return nil, err
	}

	logger.SysDebug(
		"etcd.init",
		"raw config endpoints=%v use_tls=%t dial_timeout=%s keepalive_time=%s keepalive_timeout=%s",
		cfg.Endpoints,
		cfg.UseTLS,
		cfg.DialTimeout,
		cfg.DialKeepAliveTime,
		cfg.DialKeepAliveTimeout,
	)

	endpoints := sanitizeEndpoints(cfg.Endpoints)
	if len(endpoints) == 0 {
		err := errors.New("etcd endpoints are empty")
		logger.SysError("etcd.init", err, "initialize etcd client failed")
		return nil, err
	}
	logger.SysInfo("etcd.init", "resolved etcd endpoints=%v", endpoints)

	username, password, err := resolveAuth(cfg)
	if err != nil {
		logger.SysError("etcd.auth", err, "resolve etcd auth failed")
		return nil, err
	}
	logger.SysDebug(
		"etcd.auth",
		"auth resolved username_set=%t password_set=%t",
		strings.TrimSpace(username) != "",
		strings.TrimSpace(password) != "",
	)

	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		logger.SysError("etcd.tls", err, "build etcd tls config failed")
		return nil, err
	}
	logger.SysDebug(
		"etcd.tls",
		"tls resolved enabled=%t insecure_skip_verify=%t server_name=%q ca_set=%t cert_set=%t key_set=%t",
		tlsCfg != nil,
		cfg.InsecureSkipVerify,
		strings.TrimSpace(cfg.ServerName),
		strings.TrimSpace(cfg.CA) != "",
		strings.TrimSpace(cfg.ClientCert) != "",
		strings.TrimSpace(cfg.ClientKey) != "",
	)

	logger.SysInfo(
		"etcd.init",
		"creating etcd client endpoints=%v auto_sync=%s dial_timeout=%s permit_without_stream=%t reject_old_cluster=%t",
		endpoints,
		cfg.AutoSyncInterval,
		cfg.DialTimeout,
		cfg.PermitWithoutStream,
		cfg.RejectOldCluster,
	)
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		AutoSyncInterval:     cfg.AutoSyncInterval,
		DialTimeout:          cfg.DialTimeout,
		DialKeepAliveTime:    cfg.DialKeepAliveTime,
		DialKeepAliveTimeout: cfg.DialKeepAliveTimeout,
		Username:             username,
		Password:             password,
		PermitWithoutStream:  cfg.PermitWithoutStream,
		RejectOldCluster:     cfg.RejectOldCluster,
		MaxCallSendMsgSize:   cfg.MaxCallSendMsgSize,
		MaxCallRecvMsgSize:   cfg.MaxCallRecvMsgSize,
		TLS:                  tlsCfg,
	})
	if err != nil {
		logger.SysError("etcd.init", err, "create etcd client failed")
		return nil, fmt.Errorf("create etcd client: %w", err)
	}

	timeout := cfg.DialTimeout
	if timeout <= 0 || timeout > 3*time.Second {
		timeout = 3 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if statusResp, statusErr := client.Status(pingCtx, endpoints[0]); statusErr != nil {
		logger.SysWarn("etcd.ping", "status check failed endpoint=%s err=%v", endpoints[0], statusErr)
	} else {
		logger.SysDebug(
			"etcd.ping",
			"status ok endpoint=%s leader=%d raft_term=%d raft_index=%d db_size=%d version=%s",
			endpoints[0],
			statusResp.Leader,
			statusResp.RaftTerm,
			statusResp.RaftIndex,
			statusResp.DbSize,
			statusResp.Version,
		)
	}

	logger.SysInfo("etcd.init", "etcd client initialized successfully")
	return client, nil
}

func sanitizeEndpoints(endpoints []string) []string {
	if len(endpoints) == 0 {
		return nil
	}
	uniq := make(map[string]struct{}, len(endpoints))
	out := make([]string, 0, len(endpoints))
	for _, raw := range endpoints {
		endpoint := strings.TrimSpace(raw)
		if endpoint == "" {
			continue
		}
		if _, exists := uniq[endpoint]; exists {
			continue
		}
		uniq[endpoint] = struct{}{}
		out = append(out, endpoint)
	}
	return out
}

func resolveAuth(cfg *config.EtcdCfg) (string, string, error) {
	username := strings.TrimSpace(cfg.Username)
	password := cfg.Password

	if username == "" && strings.TrimSpace(password) != "" {
		return "", "", errors.New("etcd username is required when password is provided")
	}
	if username != "" && strings.TrimSpace(password) == "" {
		return "", "", errors.New("etcd password is required when username is provided")
	}
	return username, password, nil
}

func buildTLSConfig(cfg *config.EtcdCfg) (*tls.Config, error) {
	if !cfg.UseTLS {
		return nil, nil
	}

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         strings.TrimSpace(cfg.ServerName),
	}

	caPath := strings.TrimSpace(cfg.CA)
	if caPath != "" {
		caPEM, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read etcd tls ca file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if ok := pool.AppendCertsFromPEM(caPEM); !ok {
			return nil, errors.New("invalid etcd tls ca certificate")
		}
		tlsCfg.RootCAs = pool
	}

	certPath := strings.TrimSpace(cfg.ClientCert)
	keyPath := strings.TrimSpace(cfg.ClientKey)
	if certPath == "" && keyPath == "" {
		return tlsCfg, nil
	}
	if certPath == "" || keyPath == "" {
		return nil, errors.New("both etcd tls client cert and key are required")
	}

	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load etcd tls client cert/key: %w", err)
	}
	tlsCfg.Certificates = []tls.Certificate{pair}
	return tlsCfg, nil
}
