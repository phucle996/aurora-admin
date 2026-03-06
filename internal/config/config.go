package config

import (
	keycfg "admin/internal/key"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type AppCfg struct {
	Name     string
	HostName string
	Port     int
	LogLV    string
	TimeZone string
	TLSCert  string
	TLSKey   string
	TLSCA    string
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
			TLSCert:  "/etc/aurora/certs/admin.crt",
			TLSKey:   "/etc/aurora/certs/admin.key",
			TLSCA:    "/etc/aurora/certs/ca.crt",
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
			Prefix:         keycfg.APIKeyPrefix,
			RotateInterval: 72 * time.Hour,
		},
		Telegram: TelegramCfg{
			BotToken:    getEnv("TELEGRAM_BOT_TOKEN", ""),
			ChatID:      getEnv("TELEGRAM_CHAT_ID", ""),
			BaseURL:     "https://api.telegram.org",
			HTTPTimeout: 5 * time.Second,
		},
		TokenSecret: TokenSecretCfg{
			Prefix:                keycfg.TokenSecretPrefix,
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
			Prefix: keycfg.CertStorePrefix,
		},
		Cors: CorsCfg{
			AllowOrigins: []string{},
			AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},

			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},

			ExposeHeaders:    []string{},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		},
	}
}

func LoadRuntimeFromEtcd(ctx context.Context, cli *clientv3.Client, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	values, err := loadPrefixedValues(ctx, cli, keycfg.RuntimePrefix)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return fmt.Errorf("no key found in prefix %s", keycfg.RuntimePrefix)
	}

	if cfg.App.TimeZone, err = readRequiredString(values, keycfg.RTAppTZ); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Database.URL, err = readRequiredString(values, keycfg.RTPgURL); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Database.SSLMode, err = readRequiredString(values, keycfg.RTPgSSLMode); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Redis.Addr, err = readRequiredString(values, keycfg.RTRedisAddr); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Redis.Username = readOptionalString(values, keycfg.RTRedisUser)
	cfg.Redis.Password = readOptionalString(values, keycfg.RTRedisPass)
	if cfg.Redis.DB, err = readRequiredInt(values, keycfg.RTRedisDB); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Redis.UseTLS, err = readRequiredBool(values, keycfg.RTRedisTLS); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Redis.CA = readOptionalString(values, keycfg.RTRedisCA)
	cfg.Redis.ClientKey = readOptionalString(values, keycfg.RTRedisKey)
	cfg.Redis.ClientCert = readOptionalString(values, keycfg.RTRedisCert)
	if cfg.Redis.InsecureSkipVerify, err = readRequiredBool(values, keycfg.RTRedisInsecure); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.Etcd.Endpoints, err = readRequiredSlice(values, keycfg.RTEtcdEndpoints); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.AutoSyncInterval, err = readRequiredDuration(values, keycfg.RTEtcdAutoSync); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.DialTimeout, err = readRequiredDuration(values, keycfg.RTEtcdDialTimeout); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.DialKeepAliveTime, err = readRequiredDuration(values, keycfg.RTEtcdKeepAliveTime); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.DialKeepAliveTimeout, err = readRequiredDuration(values, keycfg.RTEtcdKeepAliveTimeout); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Etcd.Username = readOptionalString(values, keycfg.RTEtcdUser)
	cfg.Etcd.Password = readOptionalString(values, keycfg.RTEtcdPass)
	if cfg.Etcd.UseTLS, err = readRequiredBool(values, keycfg.RTEtcdTLS); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	cfg.Etcd.CA = readOptionalString(values, keycfg.RTEtcdCA)
	cfg.Etcd.ClientKey = readOptionalString(values, keycfg.RTEtcdKey)
	cfg.Etcd.ClientCert = readOptionalString(values, keycfg.RTEtcdCert)
	cfg.Etcd.ServerName = readOptionalString(values, keycfg.RTEtcdServerName)
	if cfg.Etcd.InsecureSkipVerify, err = readRequiredBool(values, keycfg.RTEtcdInsecure); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.PermitWithoutStream, err = readRequiredBool(values, keycfg.RTEtcdPermitNoStream); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.RejectOldCluster, err = readRequiredBool(values, keycfg.RTEtcdRejectOldCluster); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.MaxCallSendMsgSize, err = readRequiredInt(values, keycfg.RTEtcdMaxCallSendMsgSize); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.Etcd.MaxCallRecvMsgSize, err = readRequiredInt(values, keycfg.RTEtcdMaxCallRecvMsgSize); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.APIKey.RotateInterval, err = readRequiredDuration(values, keycfg.RTAPIKeyRotateEvery); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	cfg.Telegram.BotToken = readOptionalString(values, keycfg.RTTelegramBotToken)
	cfg.Telegram.ChatID = readOptionalString(values, keycfg.RTTelegramChatID)

	if cfg.TokenSecret.AccessRotateInterval, err = readRequiredDuration(values, keycfg.RTSecretRotateAccess); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenSecret.RefreshRotateInterval, err = readRequiredDuration(values, keycfg.RTSecretRotateRefresh); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenSecret.DeviceRotateInterval, err = readRequiredDuration(values, keycfg.RTSecretRotateDevice); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	if cfg.TokenTTL.AccessTTL, err = readRequiredDuration(values, keycfg.RTTTLAccess); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenTTL.RefreshTTL, err = readRequiredDuration(values, keycfg.RTTTLRefresh); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenTTL.DeviceTTL, err = readRequiredDuration(values, keycfg.RTTTLDevice); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}
	if cfg.TokenTTL.OttTTL, err = readRequiredDuration(values, keycfg.RTTTLOTT); err != nil {
		return fmt.Errorf("invalid runtime config: %w", err)
	}

	corsValues, err := loadPrefixedValues(ctx, cli, keycfg.SharedCORSPrefix)
	if err != nil {
		return err
	}
	if len(corsValues) == 0 {
		return fmt.Errorf("no key found in prefix %s", keycfg.SharedCORSPrefix)
	}

	allowOrigins, err := readRequiredSlice(corsValues, keycfg.SharedCORSAllowOrigins)
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	allowMethods, err := readRequiredSlice(corsValues, keycfg.SharedCORSAllowMethods)
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	allowHeaders, err := readRequiredSlice(corsValues, keycfg.SharedCORSAllowHeaders)
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	exposeHeaders, err := readRequiredSlice(corsValues, keycfg.SharedCORSExposeHeader)
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	allowCredentials, err := readRequiredBool(corsValues, keycfg.SharedCORSAllowCreds)
	if err != nil {
		return fmt.Errorf("invalid cors config: %w", err)
	}
	maxAge, err := readRequiredDuration(corsValues, keycfg.SharedCORSMaxAge)
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
