package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const adminEndpointKey = "/endpoint/admin"

func SeedRuntimeToEtcdIfAbsent(ctx context.Context, cli *clientv3.Client, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cli == nil {
		return errors.New("etcd client is nil")
	}

	runtimeValues := map[string]string{
		"app/timezone":                         strings.TrimSpace(cfg.App.TimeZone),
		"postgresql/url":                       strings.TrimSpace(cfg.Database.URL),
		"postgresql/sslmode":                   strings.TrimSpace(cfg.Database.SSLMode),
		"apikey/rotate_interval":               cfg.APIKey.RotateInterval.String(),
		"redis/addr":                           strings.TrimSpace(cfg.Redis.Addr),
		"redis/username":                       strings.TrimSpace(cfg.Redis.Username),
		"redis/password":                       strings.TrimSpace(cfg.Redis.Password),
		"redis/db":                             strconv.Itoa(cfg.Redis.DB),
		"redis/use_tls":                        strconv.FormatBool(cfg.Redis.UseTLS),
		"redis/ca":                             strings.TrimSpace(cfg.Redis.CA),
		"redis/client_key":                     strings.TrimSpace(cfg.Redis.ClientKey),
		"redis/client_cert":                    strings.TrimSpace(cfg.Redis.ClientCert),
		"redis/insecure_skip_verify":           strconv.FormatBool(cfg.Redis.InsecureSkipVerify),
		"etcd/endpoints":                       encodeStringSlice(cfg.Etcd.Endpoints),
		"etcd/auto_sync_interval":              cfg.Etcd.AutoSyncInterval.String(),
		"etcd/dial_timeout":                    cfg.Etcd.DialTimeout.String(),
		"etcd/dial_keepalive_time":             cfg.Etcd.DialKeepAliveTime.String(),
		"etcd/dial_keepalive_timeout":          cfg.Etcd.DialKeepAliveTimeout.String(),
		"etcd/username":                        strings.TrimSpace(cfg.Etcd.Username),
		"etcd/password":                        strings.TrimSpace(cfg.Etcd.Password),
		"etcd/use_tls":                         strconv.FormatBool(cfg.Etcd.UseTLS),
		"etcd/ca":                              strings.TrimSpace(cfg.Etcd.CA),
		"etcd/client_key":                      strings.TrimSpace(cfg.Etcd.ClientKey),
		"etcd/client_cert":                     strings.TrimSpace(cfg.Etcd.ClientCert),
		"etcd/server_name":                     strings.TrimSpace(cfg.Etcd.ServerName),
		"etcd/insecure_skip_verify":            strconv.FormatBool(cfg.Etcd.InsecureSkipVerify),
		"etcd/permit_without_stream":           strconv.FormatBool(cfg.Etcd.PermitWithoutStream),
		"etcd/reject_old_cluster":              strconv.FormatBool(cfg.Etcd.RejectOldCluster),
		"etcd/max_call_send_msg_size":          strconv.Itoa(cfg.Etcd.MaxCallSendMsgSize),
		"etcd/max_call_recv_msg_size":          strconv.Itoa(cfg.Etcd.MaxCallRecvMsgSize),
		"telegram/enable":                      strconv.FormatBool(cfg.Telegram.Enable),
		"telegram/bot_token":                   strings.TrimSpace(cfg.Telegram.BotToken),
		"telegram/chat_id":                     strings.TrimSpace(cfg.Telegram.ChatID),
		"token_secret/access_rotate_interval":  cfg.TokenSecret.AccessRotateInterval.String(),
		"token_secret/refresh_rotate_interval": cfg.TokenSecret.RefreshRotateInterval.String(),
		"token_secret/device_rotate_interval":  cfg.TokenSecret.DeviceRotateInterval.String(),
		"token_ttl/access_ttl":                 cfg.TokenTTL.AccessTTL.String(),
		"token_ttl/refresh_ttl":                cfg.TokenTTL.RefreshTTL.String(),
		"token_ttl/device_ttl":                 cfg.TokenTTL.DeviceTTL.String(),
		"token_ttl/ott_ttl":                    cfg.TokenTTL.OttTTL.String(),
	}
	if err := seedPrefixedValuesIfAbsent(ctx, cli, runtimeConfigPrefix, runtimeValues); err != nil {
		return fmt.Errorf("seed runtime config failed: %w", err)
	}

	allowOrigins := append([]string{}, cfg.Cors.AllowOrigins...)
	if appOrigin := buildOriginFromHostPort(cfg.App.HostName, cfg.App.Port); appOrigin != "" {
		allowOrigins = appendUniqueString(allowOrigins, appOrigin)
	}

	corsValues := map[string]string{
		"allow_origins":     encodeStringSlice(allowOrigins),
		"allow_methods":     encodeStringSlice(cfg.Cors.AllowMethods),
		"allow_headers":     encodeStringSlice(cfg.Cors.AllowHeaders),
		"expose_headers":    encodeStringSlice(cfg.Cors.ExposeHeaders),
		"allow_credentials": strconv.FormatBool(cfg.Cors.AllowCredentials),
		"max_age":           cfg.Cors.MaxAge.String(),
	}
	if err := seedPrefixedValuesIfAbsent(ctx, cli, sharedCorsPrefix, corsValues); err != nil {
		return fmt.Errorf("seed shared cors config failed: %w", err)
	}
	if endpoint := buildEndpointFromHostPort(cfg.App.HostName, cfg.App.Port); endpoint != "" {
		if err := putIfAbsentCAS(ctx, cli, adminEndpointKey, "running:"+endpoint); err != nil {
			return fmt.Errorf("seed admin endpoint failed: %w", err)
		}
	}

	return nil
}

func seedPrefixedValuesIfAbsent(
	ctx context.Context,
	cli *clientv3.Client,
	prefix string,
	values map[string]string,
) error {
	cleanPrefix := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" {
		return errors.New("prefix is empty")
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, relKey := range keys {
		relKey = strings.Trim(strings.TrimSpace(relKey), "/")
		if relKey == "" {
			continue
		}

		val := strings.TrimSpace(values[relKey])
		if val == "" {
			continue
		}

		fullKey := cleanPrefix + "/" + relKey
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
