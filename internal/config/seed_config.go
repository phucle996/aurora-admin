package config

import (
	keycfg "admin/internal/key"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

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
	}
	if err := seedValuesIfAbsent(ctx, cli, runtimeValues); err != nil {
		return fmt.Errorf("seed runtime config failed: %w", err)
	}

	advertisePort := normalizeAdvertisePort(cfg.App.EndpointPort, cfg.App.Port)

	allowOrigins := append([]string{}, cfg.Cors.AllowOrigins...)
	if appOrigin := buildOriginFromHostPort(cfg.App.HostName, advertisePort); appOrigin != "" {
		allowOrigins = appendUniqueString(allowOrigins, appOrigin)
	}

	corsValues := map[string]string{
		keycfg.SharedCORSAllowOrigins: encodeStringSlice(allowOrigins),
		keycfg.SharedCORSAllowMethods: encodeStringSlice(cfg.Cors.AllowMethods),
		keycfg.SharedCORSAllowHeaders: encodeStringSlice(cfg.Cors.AllowHeaders),
		keycfg.SharedCORSExposeHeader: encodeStringSlice(cfg.Cors.ExposeHeaders),
		keycfg.SharedCORSAllowCreds:   strconv.FormatBool(cfg.Cors.AllowCredentials),
		keycfg.SharedCORSMaxAge:       cfg.Cors.MaxAge.String(),
	}
	if err := seedValuesIfAbsent(ctx, cli, corsValues); err != nil {
		return fmt.Errorf("seed shared cors config failed: %w", err)
	}
	if endpoint := buildEndpointFromHostPort(cfg.App.HostName, advertisePort); endpoint != "" {
		if err := upsertValue(ctx, cli, keycfg.EndpointAdminKey, "running:"+endpoint); err != nil {
			return fmt.Errorf("upsert admin endpoint failed: %w", err)
		}
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

func appendUniqueString(existing []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return existing
	}
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return existing
		}
	}
	return append(existing, value)
}

func buildOriginFromHostPort(rawHost string, port int) string {
	host := normalizeHost(rawHost)
	if host == "" {
		return ""
	}
	scheme := "http"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawHost)), "https://") {
		scheme = "https"
	}
	if port == 443 {
		scheme = "https"
	}
	if port <= 0 || port > 65535 {
		return ""
	}
	if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
		return scheme + "://" + host
	}
	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func buildEndpointFromHostPort(rawHost string, port int) string {
	host := normalizeHost(rawHost)
	if host == "" || port <= 0 || port > 65535 {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
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

func normalizeAdvertisePort(advertisePort int, appPort int) int {
	if advertisePort > 0 && advertisePort <= 65535 {
		return advertisePort
	}
	if appPort > 0 && appPort <= 65535 {
		return appPort
	}
	return 0
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
