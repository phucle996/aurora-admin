package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	runtimeConfigPrefix = "/runtime"
	sharedCorsPrefix    = "/shared/cors"
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

type RedisCfg struct {
	Addr               string
	Username           string
	Password           string
	DB                 int
	UseTLS             bool
	CA                 string // file path
	ClientKey          string // file path
	ClientCert         string // file path
	InsecureSkipVerify bool
}
type Config struct {
	App         AppCfg
	Etcd        EtcdCfg
	Database    DatabaseCfg
	Redis       RedisCfg
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
			HostName: getEnv("APP_HOSTNAME", "aurora-admin"),
			Port:     getEnvAsInt("APP_PORT", 3009),
			LogLV:    getEnv("APP_LOG_LEVEL", "info"),
			TimeZone: getEnv("APP_TIMEZONE", "Asia/Ho_Chi_Minh"),
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
		Database: DatabaseCfg{
			URL:     getEnv("DATABASE_URL", ""),
			SSLMode: getEnv("DB_SSLMODE", ""),
		},
		Redis: RedisCfg{
			Addr:               getEnv("REDIS_ADDR", ""),
			Username:           getEnv("REDIS_USERNAME", ""),
			Password:           getEnv("REDIS_PASSWORD", ""),
			DB:                 getEnvAsInt("REDIS_DB", 0),
			UseTLS:             getEnvAsBool("REDIS_TLS", false),
			CA:                 getEnv("REDIS_TLS_CA", ""),
			ClientKey:          getEnv("REDIS_TLS_KEY", ""),
			ClientCert:         getEnv("REDIS_TLS_CERT", ""),
			InsecureSkipVerify: getEnvAsBool("REDIS_TLS_INSECURE", false),
		},
		APIKey: APIKeyCfg{
			Prefix:         "/apikey",
			RotateInterval: 72 * time.Hour,
		},
		Telegram: TelegramCfg{
			Enable:      getEnvAsBool("TELEGRAM_ENABLE", false),
			BotToken:    getEnv("TELEGRAM_BOT_TOKEN", ""),
			ChatID:      getEnv("TELEGRAM_CHAT_ID", ""),
			BaseURL:     "https://api.telegram.org",
			HTTPTimeout: 5 * time.Second,
		},
		TokenSecret: TokenSecretCfg{
			Prefix:                "/token-secret",
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
			Prefix: "/cert-store",
		},
		Cors: CorsCfg{
			AllowOrigins: getEnvAsSlice(
				"CORS_ALLOW_ORIGINS",
				[]string{},
			),
			AllowMethods: getEnvAsSlice(
				"CORS_ALLOW_METHODS",
				[]string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
			),
			AllowHeaders: getEnvAsSlice(
				"CORS_ALLOW_HEADERS",
				[]string{"Origin", "Content-Type", "Accept", "Authorization"},
			),
			ExposeHeaders:    getEnvAsSlice("CORS_EXPOSE_HEADERS", []string{}),
			AllowCredentials: getEnvAsBool("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           getEnvAsDuration("CORS_MAX_AGE", 12*time.Hour),
		},
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

	if cfg.App.TimeZone, err = readRequiredString(values, "app/timezone"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Database.URL, err = readRequiredString(values, "postgresql/url"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Database.SSLMode, err = readRequiredString(values, "postgresql/sslmode"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Redis.Addr, err = readRequiredString(values, "redis/addr"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Redis.Username = readOptionalString(values, "redis/username")
	cfg.Redis.Password = readOptionalString(values, "redis/password")
	if cfg.Redis.DB, err = readRequiredInt(values, "redis/db"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Redis.UseTLS, err = readRequiredBool(values, "redis/use_tls"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Redis.CA = readOptionalString(values, "redis/ca")
	cfg.Redis.ClientKey = readOptionalString(values, "redis/client_key")
	cfg.Redis.ClientCert = readOptionalString(values, "redis/client_cert")
	if cfg.Redis.InsecureSkipVerify, err = readRequiredBool(values, "redis/insecure_skip_verify"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Etcd.Endpoints, err = readRequiredSlice(values, "etcd/endpoints"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.AutoSyncInterval, err = readRequiredDuration(values, "etcd/auto_sync_interval"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.DialTimeout, err = readRequiredDuration(values, "etcd/dial_timeout"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.DialKeepAliveTime, err = readRequiredDuration(values, "etcd/dial_keepalive_time"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.DialKeepAliveTimeout, err = readRequiredDuration(values, "etcd/dial_keepalive_timeout"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Etcd.Username = readOptionalString(values, "etcd/username")
	cfg.Etcd.Password = readOptionalString(values, "etcd/password")
	if cfg.Etcd.UseTLS, err = readRequiredBool(values, "etcd/use_tls"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Etcd.CA = readOptionalString(values, "etcd/ca")
	cfg.Etcd.ClientKey = readOptionalString(values, "etcd/client_key")
	cfg.Etcd.ClientCert = readOptionalString(values, "etcd/client_cert")
	cfg.Etcd.ServerName = readOptionalString(values, "etcd/server_name")
	if cfg.Etcd.InsecureSkipVerify, err = readRequiredBool(values, "etcd/insecure_skip_verify"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.PermitWithoutStream, err = readRequiredBool(values, "etcd/permit_without_stream"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.RejectOldCluster, err = readRequiredBool(values, "etcd/reject_old_cluster"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.MaxCallSendMsgSize, err = readRequiredInt(values, "etcd/max_call_send_msg_size"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.MaxCallRecvMsgSize, err = readRequiredInt(values, "etcd/max_call_recv_msg_size"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.APIKey.RotateInterval, err = readRequiredDuration(values, "apikey/rotate_interval"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Telegram.Enable, err = readRequiredBool(values, "telegram/enable"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Telegram.BotToken = readOptionalString(values, "telegram/bot_token")
	cfg.Telegram.ChatID = readOptionalString(values, "telegram/chat_id")

	if cfg.TokenSecret.AccessRotateInterval, err = readRequiredDuration(values, "token_secret/access_rotate_interval"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenSecret.RefreshRotateInterval, err = readRequiredDuration(values, "token_secret/refresh_rotate_interval"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenSecret.DeviceRotateInterval, err = readRequiredDuration(values, "token_secret/device_rotate_interval"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.TokenTTL.AccessTTL, err = readRequiredDuration(values, "token_ttl/access_ttl"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenTTL.RefreshTTL, err = readRequiredDuration(values, "token_ttl/refresh_ttl"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenTTL.DeviceTTL, err = readRequiredDuration(values, "token_ttl/device_ttl"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenTTL.OttTTL, err = readRequiredDuration(values, "token_ttl/ott_ttl"); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

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
	exposeHeaders, err := readRequiredSlice(corsValues, "expose_headers")
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
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
