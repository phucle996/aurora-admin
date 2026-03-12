package config

import (
	keycfg "admin/internal/key"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func SeedRuntimeToEtcdIfAbsent(ctx context.Context, cli *clientv3.Client, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cli == nil {
		return errors.New("etcd client is nil")
	}
	platformCipherKey := strings.TrimSpace(getEnv("PLATFORM_KUBECONFIG_CIPHER_KEY", ""))
	if platformCipherKey == "" {
		generatedKey, err := randomBase64Key(32)
		if err != nil {
			return fmt.Errorf("generate platform kubeconfig cipher key failed: %w", err)
		}
		platformCipherKey = generatedKey
	}

	runtimeValues := map[string]string{
		keycfg.RTAppTZ:                       strings.TrimSpace(cfg.App.TimeZone),
		keycfg.RTAppLogLevel:                 strings.TrimSpace(cfg.App.LogLV),
		keycfg.RTPgURL:                       strings.TrimSpace(cfg.Database.URL),
		keycfg.RTPgSSLMode:                   strings.TrimSpace(cfg.Database.SSLMode),
		keycfg.RTAPIKeyRotateEvery:           cfg.APIKey.RotateInterval.String(),
		keycfg.RTRedisAddr:                   strings.TrimSpace(cfg.Redis.Addr),
		keycfg.RTRedisUser:                   strings.TrimSpace(cfg.Redis.Username),
		keycfg.RTRedisPass:                   strings.TrimSpace(cfg.Redis.Password),
		keycfg.RTRedisDB:                     strconv.Itoa(cfg.Redis.DB),
		keycfg.RTRedisTLS:                    strconv.FormatBool(cfg.Redis.UseTLS),
		keycfg.RTRedisCA:                     strings.TrimSpace(cfg.Redis.CA),
		keycfg.RTRedisKey:                    strings.TrimSpace(cfg.Redis.ClientKey),
		keycfg.RTRedisCert:                   strings.TrimSpace(cfg.Redis.ClientCert),
		keycfg.RTRedisInsecure:               strconv.FormatBool(cfg.Redis.InsecureSkipVerify),
		keycfg.RTEtcdEndpoints:               encodeStringSlice(cfg.Etcd.Endpoints),
		keycfg.RTEtcdAutoSync:                cfg.Etcd.AutoSyncInterval.String(),
		keycfg.RTEtcdDialTimeout:             cfg.Etcd.DialTimeout.String(),
		keycfg.RTEtcdKeepAliveTime:           cfg.Etcd.DialKeepAliveTime.String(),
		keycfg.RTEtcdKeepAliveTimeout:        cfg.Etcd.DialKeepAliveTimeout.String(),
		keycfg.RTEtcdUser:                    strings.TrimSpace(cfg.Etcd.Username),
		keycfg.RTEtcdPass:                    strings.TrimSpace(cfg.Etcd.Password),
		keycfg.RTEtcdTLS:                     strconv.FormatBool(cfg.Etcd.UseTLS),
		keycfg.RTEtcdCA:                      strings.TrimSpace(cfg.Etcd.CA),
		keycfg.RTEtcdKey:                     strings.TrimSpace(cfg.Etcd.ClientKey),
		keycfg.RTEtcdCert:                    strings.TrimSpace(cfg.Etcd.ClientCert),
		keycfg.RTEtcdServerName:              strings.TrimSpace(cfg.Etcd.ServerName),
		keycfg.RTEtcdInsecure:                strconv.FormatBool(cfg.Etcd.InsecureSkipVerify),
		keycfg.RTEtcdPermitNoStream:          strconv.FormatBool(cfg.Etcd.PermitWithoutStream),
		keycfg.RTEtcdRejectOldCluster:        strconv.FormatBool(cfg.Etcd.RejectOldCluster),
		keycfg.RTEtcdMaxCallSendMsgSize:      strconv.Itoa(cfg.Etcd.MaxCallSendMsgSize),
		keycfg.RTEtcdMaxCallRecvMsgSize:      strconv.Itoa(cfg.Etcd.MaxCallRecvMsgSize),
		keycfg.RTTelegramBotToken:            strings.TrimSpace(cfg.Telegram.BotToken),
		keycfg.RTTelegramChatID:              strings.TrimSpace(cfg.Telegram.ChatID),
		keycfg.RTSecretRotateAccess:          cfg.TokenSecret.AccessRotateInterval.String(),
		keycfg.RTSecretRotateRefresh:         cfg.TokenSecret.RefreshRotateInterval.String(),
		keycfg.RTSecretRotateDevice:          cfg.TokenSecret.DeviceRotateInterval.String(),
		keycfg.RTTTLAccess:                   cfg.TokenTTL.AccessTTL.String(),
		keycfg.RTTTLRefresh:                  cfg.TokenTTL.RefreshTTL.String(),
		keycfg.RTTTLDevice:                   cfg.TokenTTL.DeviceTTL.String(),
		keycfg.RTTTLOTT:                      cfg.TokenTTL.OttTTL.String(),
		keycfg.RTSecretCachePrefix:           "aurora:token-secret",
		keycfg.RTSecretCacheChannel:          "aurora:token-secret:invalidate",
		keycfg.RTSecretPollEvery:             "10s",
		keycfg.RTPlatformKubeconfigCipherKey: platformCipherKey,
		keycfg.RuntimeAppPortKey("admin"):    strconv.Itoa(cfg.App.Port),
	}
	if err := seedValuesIfAbsent(ctx, cli, runtimeValues); err != nil {
		return fmt.Errorf("seed runtime config failed: %w", err)
	}

	agentBootstrapToken := strings.TrimSpace(getEnv("AURORA_AGENT_BOOTSTRAP_TOKEN", ""))
	if agentBootstrapToken != "" {
		clusterPolicy := strings.TrimSpace(getEnv("AURORA_AGENT_BOOTSTRAP_CLUSTER", "*"))
		sum := sha256.Sum256([]byte(agentBootstrapToken))
		now := time.Now().UTC()
		record := map[string]any{
			"token_hash":    hex.EncodeToString(sum[:]),
			"cluster_scope": clusterPolicy,
			"issued_at":     now.Format(time.RFC3339Nano),
			"expires_at":    now.Add(10 * time.Minute).Format(time.RFC3339Nano),
			"max_use":       1,
		}
		recordJSON, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			return fmt.Errorf("marshal bootstrap token seed record failed: %w", marshalErr)
		}
		tokenKey := keycfg.RuntimeAgentBootstrapTokenKey(hex.EncodeToString(sum[:]))
		if err := putIfAbsentCAS(ctx, cli, tokenKey, strings.TrimSpace(string(recordJSON))); err != nil {
			return fmt.Errorf("seed agent bootstrap token failed: %w", err)
		}
	}

	if endpoint := buildEndpointFromHost(cfg.App.HostName); endpoint != "" {
		if err := upsertValue(ctx, cli, keycfg.EndpointAdminKey, "running:"+endpoint); err != nil {
			return fmt.Errorf("upsert admin endpoint failed: %w", err)
		}
	}
	if err := upsertValue(ctx, cli, keycfg.RuntimeAppPortKey("admin"), strconv.Itoa(cfg.App.Port)); err != nil {
		return fmt.Errorf("upsert admin runtime app port failed: %w", err)
	}

	return nil
}

func seedValuesIfAbsent(ctx context.Context, cli *clientv3.Client, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, fullKey := range keys {
		fullKey = strings.TrimSpace(fullKey)
		if fullKey == "" || !strings.HasPrefix(fullKey, "/") {
			continue
		}

		val := strings.TrimSpace(values[fullKey])

		if err := putIfAbsentCAS(ctx, cli, fullKey, val); err != nil {
			return fmt.Errorf("seed key %s: %w", fullKey, err)
		}
	}
	return nil
}

func putIfAbsentCAS(ctx context.Context, cli *clientv3.Client, key, value string) error {
	_, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(key), "=", 0)).
		Then(clientv3.OpPut(key, value)).
		Commit()
	return err
}

func upsertValue(ctx context.Context, cli *clientv3.Client, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is empty")
	}
	_, err := cli.Put(ctx, key, strings.TrimSpace(value))
	return err
}

func encodeStringSlice(items []string) string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			normalized = append(normalized, item)
		}
	}

	if len(normalized) == 0 {
		return "[]"
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func buildEndpointFromHost(rawHost string) string {
	host := normalizeHost(rawHost)
	if host == "" {
		return ""
	}
	return host
}

func normalizeHost(rawHost string) string {
	host := strings.TrimSpace(rawHost)
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSuffix(host, "/")
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if idx := strings.LastIndex(host, ":"); idx >= 0 && !strings.Contains(host, "]") {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	return host
}

func randomBase64Key(size int) (string, error) {
	if size <= 0 {
		return "", fmt.Errorf("invalid key size")
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(raw), nil
}
