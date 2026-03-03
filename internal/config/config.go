package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	EtcdPrefix = "/aurora"

	runtimeConfigPrefix = EtcdPrefix + "/config"
	sharedCorsPrefix    = EtcdPrefix + "/shared/cors"
	apiKeyPrefix        = EtcdPrefix + "/apikey"
	tokenSecretPrefix   = EtcdPrefix + "/token-secret"
	certStorePrefix     = EtcdPrefix + "/cert-store"
)

type AppCfg struct {
	Name     string
	HostName string
	Port     int
	LogLV    string
	TimeZone string
}

type EtcdCfg struct {
	Endpoints []string

	AutoSyncInterval     time.Duration
	DialTimeout          time.Duration
	DialKeepAliveTime    time.Duration
	DialKeepAliveTimeout time.Duration

	Username string
	Password string

	UseTLS             bool
	CA                 string
	ClientKey          string
	ClientCert         string
	ServerName         string
	InsecureSkipVerify bool

	PermitWithoutStream bool
	RejectOldCluster    bool
	MaxCallSendMsgSize  int
	MaxCallRecvMsgSize  int
}

type DatabaseCfg struct {
	URL     string
	Schema  string
	SSLMode string
}

type APIKeyCfg struct {
	Prefix         string
	RotateInterval time.Duration
}

type TelegramCfg struct {
	Enable      bool
	BotToken    string
	ChatID      string
	BaseURL     string
	HTTPTimeout time.Duration
}

type TokenSecretCfg struct {
	Prefix                string
	AccessRotateInterval  time.Duration
	RefreshRotateInterval time.Duration
	DeviceRotateInterval  time.Duration
}

type TokenTTLCfg struct {
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	DeviceTTL  time.Duration
	OttTTL     time.Duration
}

type CertStoreCfg struct {
	Prefix string
}

type CorsCfg struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

type Config struct {
	App         AppCfg
	Etcd        EtcdCfg
	Database    DatabaseCfg
	APIKey      APIKeyCfg
	Telegram    TelegramCfg
	TokenSecret TokenSecretCfg
	TokenTTL    TokenTTLCfg
	CertStore   CertStoreCfg
	Cors        CorsCfg
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading environment variables directly")
	}

	return &Config{
		App: AppCfg{
			Name:     "Aurora Admin",
			HostName: "aurora-admin",
			Port:     3009,
			LogLV:    "info",
			TimeZone: "Asia/Ho_Chi_Minh",
		},
		Etcd: EtcdCfg{
			Endpoints:            getEnvAsSlice("ETCD_ENDPOINTS", []string{"localhost:2379"}),
			AutoSyncInterval:     getEnvAsDuration("ETCD_AUTO_SYNC_INTERVAL", 5*time.Minute),
			DialTimeout:          getEnvAsDuration("ETCD_DIAL_TIMEOUT", 5*time.Second),
			DialKeepAliveTime:    getEnvAsDuration("ETCD_DIAL_KEEPALIVE_TIME", 30*time.Second),
			DialKeepAliveTimeout: getEnvAsDuration("ETCD_DIAL_KEEPALIVE_TIMEOUT", 10*time.Second),
			Username:             getEnv("ETCD_USERNAME", ""),
			Password:             getEnv("ETCD_PASSWORD", ""),
			UseTLS:               getEnvAsBool("ETCD_TLS", false),
			CA:                   getEnv("ETCD_TLS_CA", ""),
			ClientKey:            getEnv("ETCD_TLS_KEY", ""),
			ClientCert:           getEnv("ETCD_TLS_CERT", ""),
			ServerName:           getEnv("ETCD_TLS_SERVER_NAME", ""),
			InsecureSkipVerify:   getEnvAsBool("ETCD_TLS_INSECURE", false),
			PermitWithoutStream:  getEnvAsBool("ETCD_PERMIT_WITHOUT_STREAM", false),
			RejectOldCluster:     getEnvAsBool("ETCD_REJECT_OLD_CLUSTER", false),
			MaxCallSendMsgSize:   getEnvAsInt("ETCD_MAX_CALL_SEND_MSG_SIZE", 2*1024*1024),
			MaxCallRecvMsgSize:   getEnvAsInt("ETCD_MAX_CALL_RECV_MSG_SIZE", 2*1024*1024),
		},
		APIKey: APIKeyCfg{
			Prefix:         apiKeyPrefix,
			RotateInterval: 72 * time.Hour,
		},
		Telegram: TelegramCfg{
			Enable:      false,
			BotToken:    "",
			ChatID:      "",
			BaseURL:     "https://api.telegram.org",
			HTTPTimeout: 5 * time.Second,
		},
		TokenSecret: TokenSecretCfg{
			Prefix:                tokenSecretPrefix,
			AccessRotateInterval:  72 * time.Hour,
			RefreshRotateInterval: 7 * 24 * time.Hour,
			DeviceRotateInterval:  14 * 24 * time.Hour,
		},
		TokenTTL: TokenTTLCfg{
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 168 * time.Hour,
			DeviceTTL:  15 * time.Minute,
			OttTTL:     15 * time.Minute,
		},
		CertStore: CertStoreCfg{
			Prefix: certStorePrefix,
		},
		Cors: CorsCfg{},
	}
}

func LoadRuntimeFromEtcd(ctx context.Context, cli *clientv3.Client, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	values, err := loadPrefixedValues(ctx, cli, runtimeConfigPrefix)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return fmt.Errorf("no key found in prefix %s", runtimeConfigPrefix)
	}

	cfg.App.HostName = readString(values, "app/hostname", cfg.App.HostName)
	cfg.App.Port = readInt(values, "app/port", cfg.App.Port)
	cfg.App.TimeZone = readString(values, "app/timezone", cfg.App.TimeZone)

	cfg.Database.URL = readString(values, "database/url", cfg.Database.URL)
	cfg.Database.Schema = readString(values, "database/schema", cfg.Database.Schema)
	cfg.Database.SSLMode = readString(values, "database/sslmode", cfg.Database.SSLMode)

	cfg.Telegram.Enable = readBool(values, "telegram/enable", cfg.Telegram.Enable)
	cfg.Telegram.BotToken = readString(values, "telegram/bot_token", cfg.Telegram.BotToken)
	cfg.Telegram.ChatID = readString(values, "telegram/chat_id", cfg.Telegram.ChatID)

	corsValues, err := loadPrefixedValues(ctx, cli, sharedCorsPrefix)
	if err != nil {
		return err
	}
	if len(corsValues) == 0 {
		return fmt.Errorf("no key found in prefix %s", sharedCorsPrefix)
	}

	allowOrigins, err := readRequiredSlice(corsValues, "allow_origins")
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	allowMethods, err := readRequiredSlice(corsValues, "allow_methods")
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	allowHeaders, err := readRequiredSlice(corsValues, "allow_headers")
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	exposeHeaders := readSlice(corsValues, "expose_headers", []string{})
	allowCredentials, err := readRequiredBool(corsValues, "allow_credentials")
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	maxAge, err := readRequiredDuration(corsValues, "max_age")
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}

	cfg.Cors.AllowOrigins = allowOrigins
	cfg.Cors.AllowMethods = allowMethods
	cfg.Cors.AllowHeaders = allowHeaders
	cfg.Cors.ExposeHeaders = exposeHeaders
	cfg.Cors.AllowCredentials = allowCredentials
	cfg.Cors.MaxAge = maxAge

	return nil
}

func loadPrefixedValues(ctx context.Context, cli *clientv3.Client, prefix string) (map[string]string, error) {
	if cli == nil {
		return nil, errors.New("etcd client is nil")
	}
	cleanPrefix := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" {
		return nil, errors.New("prefix is empty")
	}

	resp, err := cli.Get(ctx, cleanPrefix+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		rawKey := strings.TrimSpace(string(kv.Key))
		relative := strings.TrimPrefix(rawKey, cleanPrefix+"/")
		relative = strings.TrimSpace(relative)
		if relative == "" {
			continue
		}
		out[relative] = strings.TrimSpace(string(kv.Value))
	}
	return out, nil
}

func readString(values map[string]string, key, fallback string) string {
	v, ok := values[key]
	if !ok {
		return fallback
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func readInt(values map[string]string, key string, fallback int) int {
	v, ok := values[key]
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return n
}

func readBool(values map[string]string, key string, fallback bool) bool {
	v, ok := values[key]
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return b
}

func readDuration(values map[string]string, key string, fallback time.Duration) time.Duration {
	v, ok := values[key]
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return d
}

func readSlice(values map[string]string, key string, fallback []string) []string {
	v, ok := values[key]
	if !ok {
		return fallback
	}
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return fallback
	}

	if strings.HasPrefix(trimmed, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				item = strings.TrimSpace(item)
				if item != "" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func readRequiredSlice(values map[string]string, key string) ([]string, error) {
	v, ok := values[key]
	if !ok {
		return nil, fmt.Errorf("missing key %q", key)
	}
	parsed := readSlice(values, key, nil)
	if len(parsed) == 0 {
		return nil, fmt.Errorf("key %q must not be empty (raw=%q)", key, strings.TrimSpace(v))
	}
	return parsed, nil
}

func readRequiredBool(values map[string]string, key string) (bool, error) {
	v, ok := values[key]
	if !ok {
		return false, fmt.Errorf("missing key %q", key)
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, fmt.Errorf("invalid bool for key %q: %w", key, err)
	}
	return b, nil
}

func readRequiredDuration(values map[string]string, key string) (time.Duration, error) {
	v, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("missing key %q", key)
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid duration for key %q: %w", key, err)
	}
	return d, nil
}
