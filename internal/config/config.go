package config

import (
	"log"
	"time"

	"github.com/joho/godotenv"
)

type AppCfg struct {
	Name     string
	Host     string
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
	CA                 string // file path
	ClientKey          string // file path
	ClientCert         string // file path
	ServerName         string
	InsecureSkipVerify bool

	PermitWithoutStream bool
	RejectOldCluster    bool
	MaxCallSendMsgSize  int
	MaxCallRecvMsgSize  int

	// etcd supports username/password auth; this block lets deployments
	// map SASL-PLAIN style envs into that auth flow.
	SASLEnable    bool
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

type APIKeyCfg struct {
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
	RotateCheckInterval   time.Duration
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
	APIKey      APIKeyCfg
	Telegram    TelegramCfg
	TokenSecret TokenSecretCfg
	CertStore   CertStoreCfg
	Cors        CorsCfg
}

func LoadConfig() *Config {
	// load .env file (ignore if missing, can use env vars directly)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading environment variables directly")
	}

	return &Config{
		App: AppCfg{
			Name:     "Aurora Admin",
			Host:     getEnv("APP_HOST", ""),
			Port:     getEnvAsInt("APP_PORT", 3009),
			LogLV:    getEnv("APP_LOG_LEVEL", ""),
			TimeZone: getEnv("APP_TIMEZONE", "Asia/Ho_Chi_Minh"),
		},

		Etcd: EtcdCfg{
			Endpoints:            getEnvAsSlice("ETCD_ENDPOINTS", []string{"localhost:2379"}),
			AutoSyncInterval:     getEnvAsDuration("ETCD_AUTO_SYNC_INTERVAL", 5*time.Minute),
			DialTimeout:          getEnvAsDuration("ETCD_DIAL_TIMEOUT", 5*time.Second),
			DialKeepAliveTime:    getEnvAsDuration("ETCD_DIAL_KEEPALIVE_TIME", 30*time.Second),
			DialKeepAliveTimeout: getEnvAsDuration("ETCD_DIAL_KEEPALIVE_TIMEOUT", 10*time.Second),

			Username: getEnv("ETCD_USERNAME", ""),
			Password: getEnv("ETCD_PASSWORD", ""),

			UseTLS:             getEnvAsBool("ETCD_TLS", false),
			CA:                 getEnv("ETCD_TLS_CA", ""),
			ClientKey:          getEnv("ETCD_TLS_KEY", ""),
			ClientCert:         getEnv("ETCD_TLS_CERT", ""),
			ServerName:         getEnv("ETCD_TLS_SERVER_NAME", ""),
			InsecureSkipVerify: getEnvAsBool("ETCD_TLS_INSECURE", false),

			PermitWithoutStream: getEnvAsBool("ETCD_PERMIT_WITHOUT_STREAM", false),
			RejectOldCluster:    getEnvAsBool("ETCD_REJECT_OLD_CLUSTER", false),
			MaxCallSendMsgSize:  getEnvAsInt("ETCD_MAX_CALL_SEND_MSG_SIZE", 2*1024*1024),
			MaxCallRecvMsgSize:  getEnvAsInt("ETCD_MAX_CALL_RECV_MSG_SIZE", 2*1024*1024),

			SASLEnable:    getEnvAsBool("ETCD_SASL_ENABLE", false),
			SASLMechanism: getEnv("ETCD_SASL_MECHANISM", "PLAIN"),
			SASLUsername:  getEnv("ETCD_SASL_USERNAME", ""),
			SASLPassword:  getEnv("ETCD_SASL_PASSWORD", ""),
		},
		APIKey: APIKeyCfg{
			RotateInterval: getEnvAsDuration("ADMIN_APIKEY_ROTATE_INTERVAL", 72*time.Hour),
		},
		Telegram: TelegramCfg{
			Enable:      getEnvAsBool("TELEGRAM_ENABLE", true),
			BotToken:    getEnv("TELEGRAM_BOT_TOKEN", "8292915788:AAFLU6VAF0sp88wd8yyqW1F-bRDpICQMN-o"),
			ChatID:      getEnv("TELEGRAM_CHAT_ID", "-1003696373127"),
			BaseURL:     getEnv("TELEGRAM_BASE_URL", "https://api.telegram.org"),
			HTTPTimeout: getEnvAsDuration("TELEGRAM_HTTP_TIMEOUT", 5*time.Second),
		},
		TokenSecret: TokenSecretCfg{
			Prefix:                getEnv("ADMIN_TOKEN_SECRET_PREFIX", "/admin/token-secret"),
			AccessRotateInterval:  getEnvAsDuration("ADMIN_ACCESS_TOKEN_ROTATE_INTERVAL", 72*time.Hour),
			RefreshRotateInterval: getEnvAsDuration("ADMIN_REFRESH_TOKEN_ROTATE_INTERVAL", 7*24*time.Hour),
			DeviceRotateInterval:  getEnvAsDuration("ADMIN_DEVICE_TOKEN_ROTATE_INTERVAL", 14*24*time.Hour),
			RotateCheckInterval:   getEnvAsDuration("ADMIN_TOKEN_SECRET_ROTATE_CHECK_INTERVAL", time.Hour),
		},
		CertStore: CertStoreCfg{
			Prefix: getEnv("ADMIN_CERT_STORE_PREFIX", ""),
		},

		Cors: CorsCfg{
			AllowOrigins: getEnvAsSlice("CORS_ALLOW_ORIGINS", []string{"http://localhost:5173"}),
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
