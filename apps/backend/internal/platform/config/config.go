package config

import (
	"cmp"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                              string
	HTTPAddr                            string
	LogLevel                            string
	LogLevelValue                       slog.Level
	LogFormat                           string
	DatabaseURL                         string
	MigrationsDir                       string
	CookieSecure                        bool
	PaymentProvider                     string
	AppBaseURL                          string
	TKassaTerminalKey                   string
	TKassaPassword                      string
	TKassaBaseURL                       string
	TKassaTimeout                       time.Duration
	TKassaMaxRetries                    int
	TKassaRetryBaseDelay                time.Duration
	TKassaRetryMaxDelay                 time.Duration
	DaDataAPIKey                        string
	DaDataSecretKey                     string
	DaDataBaseURL                       string
	DaDataTimeout                       time.Duration
	EncryptionKey                       string
	BillingWorkerInterval               time.Duration
	PaymentReconciliationWorkerInterval time.Duration
	OverdueOperationWorkerInterval      time.Duration
	LogSuccessfulRequests               bool
	TariffCacheTTL                      time.Duration
	TrustedProxies                      []string
	RateLimit                           RateLimit
	DBPool                              DBPoolConfig
	PhotoStorageEndpoint                string
	PhotoStorageRegion                  string
	PhotoStorageBucket                  string
	PhotoStorageAccessKey               string
	PhotoStorageSecretKey               string
	PhotoStoragePublicBaseURL           string
	PhotoStoragePathStyle               bool
	PhotoStorageProvider                string
	PhotoStorageS3Enabled               bool
	EmailSender                         string
	EmailTemplatesDir                   string
	SMTPHost                            string
	SMTPPort                            string
	SMTPUser                            string
	SMTPPass                            string
	SMTPFrom                            string
	SMTPFromName                        string
	SMTPTimeout                         time.Duration
	OTelServiceName                     string
	OTelEnabled                         bool
	OTelTraceSampler                    float64
	OTelOTLPEndpoint                    string
}

// RateLimit holds per-key rate-limiting configuration.
type RateLimit struct {
	IPRPS                     float64
	IPBurst                   int
	EmailSendPerHour          int
	EmailVerifyPer15Min       int
	PhoneChangeSendPerHour    int
	PhoneChangeVerifyPer15Min int
}

// DBPoolConfig holds PostgreSQL connection pool settings.
type DBPoolConfig struct {
	MaxConns                        int32
	MinConns                        int32
	MaxConnLifetime                 time.Duration
	MaxConnIdleTime                 time.Duration
	HealthCheckPeriod               time.Duration
	StatementTimeout                time.Duration
	IdleInTransactionSessionTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:            os.Getenv("APP_ENV"),
		HTTPAddr:          os.Getenv("HTTP_ADDR"),
		LogLevel:          os.Getenv("LOG_LEVEL"),
		LogFormat:         os.Getenv("LOG_FORMAT"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		MigrationsDir:     os.Getenv("MIGRATIONS_DIR"),
		EmailSender:       os.Getenv("EMAIL_SENDER"),
		EmailTemplatesDir: os.Getenv("EMAIL_TEMPLATES_DIR"),
		SMTPHost:          os.Getenv("SMTP_HOST"),
		SMTPPort:          os.Getenv("SMTP_PORT"),
		SMTPUser:          os.Getenv("SMTP_USER"),
		SMTPPass:          os.Getenv("SMTP_PASS"),
		SMTPFrom:          os.Getenv("SMTP_FROM"),
		SMTPFromName:      os.Getenv("SMTP_FROM_NAME"),
		PaymentProvider:   os.Getenv("PAYMENT_PROVIDER"),
		SMTPTimeout:       10 * time.Second,
		AppBaseURL:        os.Getenv("APP_BASE_URL"),
		TKassaTerminalKey: os.Getenv("T_KASSA_TERMINAL_KEY"),
		TKassaPassword:    os.Getenv("T_KASSA_PASSWORD"),
		TKassaBaseURL:     os.Getenv("T_KASSA_BASE_URL"),
		DaDataAPIKey:      os.Getenv("DADATA_API_KEY"),
		DaDataSecretKey:   os.Getenv("DADATA_SECRET_KEY"),
		DaDataBaseURL:     os.Getenv("DADATA_BASE_URL"),
		EncryptionKey:     os.Getenv("ENCRYPTION_KEY"),
		OTelServiceName:   cmp.Or(os.Getenv("OTEL_SERVICE_NAME"), "arenda-api"),
		OTelEnabled: (os.Getenv("OTEL_TRACES_EXPORTER") != "" && os.Getenv("OTEL_TRACES_EXPORTER") != "none") ||
			(os.Getenv("OTEL_METRICS_EXPORTER") != "" && os.Getenv("OTEL_METRICS_EXPORTER") != "none"),
		OTelOTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}

	sampler, err := parseFloatEnv("OTEL_TRACES_SAMPLER_ARG", 1.0)
	if err != nil {
		return Config{}, fmt.Errorf("invalid OTEL_TRACES_SAMPLER_ARG %q: %w", os.Getenv("OTEL_TRACES_SAMPLER_ARG"), err)
	}
	cfg.OTelTraceSampler = sampler

	if cfg.OTelEnabled && cfg.OTelOTLPEndpoint == "" {
		return Config{}, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_TRACES_EXPORTER or OTEL_METRICS_EXPORTER is set")
	}

	if cfg.AppEnv == "" {
		return Config{}, fmt.Errorf("APP_ENV is required")
	}
	allowedEnvs := map[string]bool{"local": true, "dev": true, "staging": true, "production": true}
	if !allowedEnvs[cfg.AppEnv] {
		return Config{}, fmt.Errorf("invalid APP_ENV %q: must be one of local, dev, staging, production", cfg.AppEnv)
	}

	if cfg.DaDataBaseURL == "" {
		cfg.DaDataBaseURL = "https://suggestions.dadata.ru/suggestions/api/4_1/rs/suggest/address"
	}
	if dadataURL, err := url.Parse(cfg.DaDataBaseURL); err != nil {
		return Config{}, fmt.Errorf("invalid DADATA_BASE_URL %q: %w", cfg.DaDataBaseURL, err)
	} else if dadataURL.Scheme != "http" && dadataURL.Scheme != "https" {
		return Config{}, fmt.Errorf("invalid DADATA_BASE_URL %q: scheme must be http or https", cfg.DaDataBaseURL)
	}

	cfg.DaDataTimeout = 10 * time.Second
	if v := os.Getenv("DADATA_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DADATA_TIMEOUT %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("DADATA_TIMEOUT must be positive")
		}
		cfg.DaDataTimeout = d
	}

	if cfg.DaDataAPIKey == "" {
		return Config{}, fmt.Errorf("DADATA_API_KEY is required")
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if err := cfg.LogLevelValue.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return Config{}, fmt.Errorf("invalid LOG_LEVEL %q: %w", cfg.LogLevel, err)
	}

	if cfg.LogFormat == "" {
		switch cfg.AppEnv {
		case "local", "dev":
			cfg.LogFormat = "pretty"
		default:
			cfg.LogFormat = "json"
		}
	}
	allowedFormats := map[string]bool{"json": true, "pretty": true}
	if !allowedFormats[cfg.LogFormat] {
		return Config{}, fmt.Errorf("invalid LOG_FORMAT %q: must be json or pretty", cfg.LogFormat)
	}

	cfg.LogSuccessfulRequests = true
	if v := os.Getenv("LOG_SUCCESSFUL_REQUESTS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid LOG_SUCCESSFUL_REQUESTS %q: %w", v, err)
		}
		cfg.LogSuccessfulRequests = b
	}

	cookieSecure := os.Getenv("COOKIE_SECURE")
	cookieSecureExplicit := false
	if cookieSecure != "" {
		v, err := strconv.ParseBool(cookieSecure)
		if err != nil {
			return Config{}, fmt.Errorf("invalid COOKIE_SECURE %q: %w", cookieSecure, err)
		}
		cfg.CookieSecure = v
		cookieSecureExplicit = true
	}
	if !cookieSecureExplicit {
		switch cfg.AppEnv {
		case "local":
			cfg.CookieSecure = false
		default:
			cfg.CookieSecure = true
		}
	}
	if !cfg.CookieSecure && cfg.AppEnv != "local" && cookieSecureExplicit {
		return Config{}, fmt.Errorf("COOKIE_SECURE=false is not allowed for APP_ENV=%s", cfg.AppEnv)
	}

	cfg.RateLimit = RateLimit{
		IPRPS:                     20,
		IPBurst:                   40,
		EmailSendPerHour:          60,
		EmailVerifyPer15Min:       30,
		PhoneChangeSendPerHour:    5,
		PhoneChangeVerifyPer15Min: 10,
	}
	if v := os.Getenv("RATE_LIMIT_IP_RPS"); v != "" {
		rps, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_IP_RPS %q: %w", v, err)
		}
		cfg.RateLimit.IPRPS = rps
	}
	if v := os.Getenv("RATE_LIMIT_IP_BURST"); v != "" {
		burst, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_IP_BURST %q: %w", v, err)
		}
		cfg.RateLimit.IPBurst = burst
	}
	if v := os.Getenv("RATE_LIMIT_EMAIL_SEND_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_EMAIL_SEND_PER_HOUR %q: %w", v, err)
		}
		cfg.RateLimit.EmailSendPerHour = n
	}
	if v := os.Getenv("RATE_LIMIT_EMAIL_VERIFY_PER_15MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_EMAIL_VERIFY_PER_15MIN %q: %w", v, err)
		}
		cfg.RateLimit.EmailVerifyPer15Min = n
	}
	if v := os.Getenv("RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR %q: %w", v, err)
		}
		cfg.RateLimit.PhoneChangeSendPerHour = n
	} else if cfg.RateLimit.PhoneChangeSendPerHour <= 0 {
		cfg.RateLimit.PhoneChangeSendPerHour = 5
	}
	if v := os.Getenv("RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN %q: %w", v, err)
		}
		cfg.RateLimit.PhoneChangeVerifyPer15Min = n
	} else if cfg.RateLimit.PhoneChangeVerifyPer15Min <= 0 {
		cfg.RateLimit.PhoneChangeVerifyPer15Min = 10
	}
	if cfg.RateLimit.IPRPS <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_IP_RPS must be positive")
	}
	if cfg.RateLimit.IPBurst <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_IP_BURST must be positive")
	}
	if cfg.RateLimit.EmailSendPerHour <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_EMAIL_SEND_PER_HOUR must be positive")
	}
	if cfg.RateLimit.EmailVerifyPer15Min <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_EMAIL_VERIFY_PER_15MIN must be positive")
	}
	if cfg.RateLimit.PhoneChangeSendPerHour <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR must be positive")
	}
	if cfg.RateLimit.PhoneChangeVerifyPer15Min <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN must be positive")
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://") && !strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		return Config{}, fmt.Errorf("invalid DATABASE_URL: must start with postgres:// or postgresql://")
	}
	if cfg.MigrationsDir == "" {
		return Config{}, fmt.Errorf("MIGRATIONS_DIR is required")
	}

	cfg.DBPool = DBPoolConfig{
		MaxConns:                        64,
		MinConns:                        16,
		MaxConnLifetime:                 30 * time.Minute,
		MaxConnIdleTime:                 5 * time.Minute,
		HealthCheckPeriod:               30 * time.Second,
		StatementTimeout:                30 * time.Second,
		IdleInTransactionSessionTimeout: 60 * time.Second,
	}
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_MAX_CONNS %q: %w", v, err)
		}
		cfg.DBPool.MaxConns = int32(n)
	}
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_MIN_CONNS %q: %w", v, err)
		}
		cfg.DBPool.MinConns = int32(n)
	}
	if v := os.Getenv("DB_MAX_CONN_LIFETIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_MAX_CONN_LIFETIME %q: %w", v, err)
		}
		cfg.DBPool.MaxConnLifetime = d
	}
	if v := os.Getenv("DB_MAX_CONN_IDLE_TIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_MAX_CONN_IDLE_TIME %q: %w", v, err)
		}
		cfg.DBPool.MaxConnIdleTime = d
	}
	if v := os.Getenv("DB_STATEMENT_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_STATEMENT_TIMEOUT %q: %w", v, err)
		}
		cfg.DBPool.StatementTimeout = d
	}
	if v := os.Getenv("DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT %q: %w", v, err)
		}
		cfg.DBPool.IdleInTransactionSessionTimeout = d
	}
	if v := os.Getenv("DB_HEALTH_CHECK_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DB_HEALTH_CHECK_PERIOD %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("DB_HEALTH_CHECK_PERIOD must be positive")
		}
		cfg.DBPool.HealthCheckPeriod = d
	}
	if cfg.DBPool.MaxConns <= 0 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be positive")
	}
	if cfg.DBPool.MinConns < 0 {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must be non-negative")
	}
	if cfg.DBPool.MinConns > cfg.DBPool.MaxConns {
		return Config{}, fmt.Errorf("DB_MIN_CONNS must not exceed DB_MAX_CONNS")
	}

	allowedEmailSenders := map[string]bool{"": true, "fake": true, "smtp": true}
	if !allowedEmailSenders[cfg.EmailSender] {
		return Config{}, fmt.Errorf("invalid EMAIL_SENDER %q: must be empty, fake, or smtp", cfg.EmailSender)
	}
	if cfg.EmailSender == "" {
		if cfg.AppEnv == "local" || cfg.AppEnv == "dev" {
			cfg.EmailSender = "fake"
		} else {
			return Config{}, fmt.Errorf("EMAIL_SENDER is required for APP_ENV=%s", cfg.AppEnv)
		}
	}
	if cfg.EmailSender == "fake" && cfg.AppEnv != "local" && cfg.AppEnv != "dev" {
		return Config{}, fmt.Errorf("EMAIL_SENDER=fake is not allowed for APP_ENV=%s", cfg.AppEnv)
	}
	if cfg.EmailTemplatesDir == "" {
		cfg.EmailTemplatesDir = "apps/backend/templates/email"
	}
	if cfg.EmailSender == "smtp" && cfg.AppEnv != "local" && cfg.AppEnv != "dev" {
		if !filepath.IsAbs(cfg.EmailTemplatesDir) {
			return Config{}, fmt.Errorf("EMAIL_TEMPLATES_DIR must be an absolute path in %s environment", cfg.AppEnv)
		}
	}
	if cfg.EmailSender == "smtp" {
		if cfg.SMTPHost == "" {
			return Config{}, fmt.Errorf("SMTP_HOST is required when EMAIL_SENDER=smtp")
		}
		if cfg.SMTPPort == "" {
			return Config{}, fmt.Errorf("SMTP_PORT is required when EMAIL_SENDER=smtp")
		}
		if cfg.SMTPFrom == "" {
			return Config{}, fmt.Errorf("SMTP_FROM is required when EMAIL_SENDER=smtp")
		}
		if v := os.Getenv("SMTP_TIMEOUT"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid SMTP_TIMEOUT %q: %w", v, err)
			}
			if d <= 0 {
				return Config{}, fmt.Errorf("SMTP_TIMEOUT must be positive")
			}
			cfg.SMTPTimeout = d
		}
	}

	if cfg.PaymentProvider == "" {
		if cfg.AppEnv == "local" {
			cfg.PaymentProvider = "fake"
		} else {
			return Config{}, fmt.Errorf("PAYMENT_PROVIDER is required for APP_ENV=%s", cfg.AppEnv)
		}
	}
	allowedPaymentProviders := map[string]bool{"fake": true, "tkassa": true}
	if !allowedPaymentProviders[cfg.PaymentProvider] {
		return Config{}, fmt.Errorf("invalid PAYMENT_PROVIDER %q: must be fake or tkassa", cfg.PaymentProvider)
	}
	if cfg.AppEnv != "local" && cfg.AppEnv != "dev" && cfg.PaymentProvider == "fake" {
		return Config{}, fmt.Errorf("PAYMENT_PROVIDER=fake is not allowed for APP_ENV=%s", cfg.AppEnv)
	}
	if cfg.PaymentProvider == "fake" {
		if cfg.AppBaseURL == "" {
			return Config{}, fmt.Errorf("APP_BASE_URL is required when PAYMENT_PROVIDER=fake")
		}
		u, err := url.Parse(cfg.AppBaseURL)
		if err != nil {
			return Config{}, fmt.Errorf("invalid APP_BASE_URL %q: %w", cfg.AppBaseURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return Config{}, fmt.Errorf("invalid APP_BASE_URL %q: scheme must be http or https", cfg.AppBaseURL)
		}
	}
	if cfg.PaymentProvider == "tkassa" {
		if cfg.TKassaTerminalKey == "" {
			return Config{}, fmt.Errorf("T_KASSA_TERMINAL_KEY is required when PAYMENT_PROVIDER=tkassa")
		}
		if cfg.TKassaPassword == "" {
			return Config{}, fmt.Errorf("T_KASSA_PASSWORD is required when PAYMENT_PROVIDER=tkassa")
		}
		if cfg.AppBaseURL == "" {
			return Config{}, fmt.Errorf("APP_BASE_URL is required when PAYMENT_PROVIDER=tkassa")
		}
		u, err := url.Parse(cfg.AppBaseURL)
		if err != nil {
			return Config{}, fmt.Errorf("invalid APP_BASE_URL %q: %w", cfg.AppBaseURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return Config{}, fmt.Errorf("invalid APP_BASE_URL %q: scheme must be http or https", cfg.AppBaseURL)
		}
		if cfg.AppEnv != "local" && cfg.AppEnv != "dev" && u.Scheme != "https" {
			return Config{}, fmt.Errorf("invalid APP_BASE_URL %q: non-local/dev environments must use https", cfg.AppBaseURL)
		}
		if cfg.TKassaBaseURL == "" {
			if cfg.AppEnv != "local" && cfg.AppEnv != "dev" {
				return Config{}, fmt.Errorf("T_KASSA_BASE_URL is required when PAYMENT_PROVIDER=tkassa for APP_ENV=%s", cfg.AppEnv)
			}
		} else {
			tku, err := url.Parse(cfg.TKassaBaseURL)
			if err != nil {
				return Config{}, fmt.Errorf("invalid T_KASSA_BASE_URL %q: %w", cfg.TKassaBaseURL, err)
			}
			if tku.Scheme != "http" && tku.Scheme != "https" {
				return Config{}, fmt.Errorf("invalid T_KASSA_BASE_URL %q: scheme must be http or https", cfg.TKassaBaseURL)
			}
			if cfg.AppEnv != "local" && cfg.AppEnv != "dev" {
				if tku.Scheme != "https" {
					return Config{}, fmt.Errorf("invalid T_KASSA_BASE_URL %q: non-local/dev environments must use https", cfg.TKassaBaseURL)
				}
				host := strings.ToLower(tku.Hostname())
				if host != "securepay.tinkoff.ru" {
					return Config{}, fmt.Errorf("invalid T_KASSA_BASE_URL %q: production T-Kassa base URL must be https://securepay.tinkoff.ru/v2/", cfg.TKassaBaseURL)
				}
				if strings.TrimSuffix(tku.Path, "/") != "/v2" {
					return Config{}, fmt.Errorf("invalid T_KASSA_BASE_URL %q: path must be /v2/", cfg.TKassaBaseURL)
				}
			}
		}
		cfg.TKassaTimeout = 30 * time.Second
		if v := os.Getenv("T_KASSA_TIMEOUT"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid T_KASSA_TIMEOUT %q: %w", v, err)
			}
			if d <= 0 {
				return Config{}, fmt.Errorf("T_KASSA_TIMEOUT must be positive")
			}
			cfg.TKassaTimeout = d
		}

		cfg.TKassaMaxRetries = 3
		if v := os.Getenv("T_KASSA_MAX_RETRIES"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid T_KASSA_MAX_RETRIES %q: %w", v, err)
			}
			if n < 0 {
				return Config{}, fmt.Errorf("T_KASSA_MAX_RETRIES must be non-negative")
			}
			cfg.TKassaMaxRetries = n
		}

		cfg.TKassaRetryBaseDelay = 500 * time.Millisecond
		if v := os.Getenv("T_KASSA_RETRY_BASE_DELAY"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid T_KASSA_RETRY_BASE_DELAY %q: %w", v, err)
			}
			if d <= 0 {
				return Config{}, fmt.Errorf("T_KASSA_RETRY_BASE_DELAY must be positive")
			}
			cfg.TKassaRetryBaseDelay = d
		}

		cfg.TKassaRetryMaxDelay = 5 * time.Second
		if v := os.Getenv("T_KASSA_RETRY_MAX_DELAY"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid T_KASSA_RETRY_MAX_DELAY %q: %w", v, err)
			}
			if d <= 0 {
				return Config{}, fmt.Errorf("T_KASSA_RETRY_MAX_DELAY must be positive")
			}
			cfg.TKassaRetryMaxDelay = d
		}
	}

	if cfg.AppEnv != "local" && cfg.EncryptionKey == "" {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY is required for APP_ENV=%s", cfg.AppEnv)
	}

	cfg.PhotoStorageEndpoint = os.Getenv("REGRU_S3_ENDPOINT")
	cfg.PhotoStorageRegion = os.Getenv("REGRU_S3_REGION")
	cfg.PhotoStorageBucket = os.Getenv("REGRU_S3_BUCKET")
	cfg.PhotoStorageAccessKey = os.Getenv("REGRU_S3_ACCESS_KEY")
	cfg.PhotoStorageSecretKey = os.Getenv("REGRU_S3_SECRET_KEY")
	cfg.PhotoStoragePublicBaseURL = os.Getenv("REGRU_S3_PUBLIC_BASE_URL")
	cfg.PhotoStorageProvider = strings.ToLower(strings.TrimSpace(os.Getenv("PHOTO_STORAGE_PROVIDER")))

	cfg.PhotoStoragePathStyle = true
	if v := os.Getenv("REGRU_S3_PATH_STYLE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REGRU_S3_PATH_STYLE %q: %w", v, err)
		}
		cfg.PhotoStoragePathStyle = b
	}

	s3Fields := []string{
		cfg.PhotoStorageEndpoint,
		cfg.PhotoStorageBucket,
		cfg.PhotoStorageAccessKey,
		cfg.PhotoStorageSecretKey,
		cfg.PhotoStoragePublicBaseURL,
	}
	s3Complete := !slices.Contains(s3Fields, "")
	if cfg.PhotoStorageProvider == "" {
		if cfg.AppEnv == "local" && !s3Complete {
			cfg.PhotoStorageProvider = "fake"
		} else {
			cfg.PhotoStorageProvider = "s3"
		}
	}
	switch cfg.PhotoStorageProvider {
	case "fake":
		cfg.PhotoStorageS3Enabled = false
		cfg.PhotoStoragePublicBaseURL = strings.TrimRight(cfg.AppBaseURL, "/") + "/uploads"
	case "s3":
		if !s3Complete {
			return Config{}, fmt.Errorf("REGRU_S3_ENDPOINT, REGRU_S3_BUCKET, REGRU_S3_ACCESS_KEY, REGRU_S3_SECRET_KEY and REGRU_S3_PUBLIC_BASE_URL are required for PHOTO_STORAGE_PROVIDER=s3 and APP_ENV=%s; set PHOTO_STORAGE_PROVIDER=fake only for temporary launches without photo uploads", cfg.AppEnv)
		}
		cfg.PhotoStorageS3Enabled = true
	default:
		return Config{}, fmt.Errorf("invalid PHOTO_STORAGE_PROVIDER %q: must be fake or s3", cfg.PhotoStorageProvider)
	}

	cfg.BillingWorkerInterval = time.Hour
	if v := os.Getenv("BILLING_WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid BILLING_WORKER_INTERVAL %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("BILLING_WORKER_INTERVAL must be positive")
		}
		cfg.BillingWorkerInterval = d
	}

	cfg.PaymentReconciliationWorkerInterval = 5 * time.Minute
	if v := os.Getenv("PAYMENT_RECONCILIATION_WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PAYMENT_RECONCILIATION_WORKER_INTERVAL %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("PAYMENT_RECONCILIATION_WORKER_INTERVAL must be positive")
		}
		cfg.PaymentReconciliationWorkerInterval = d
	}

	cfg.OverdueOperationWorkerInterval = 24 * time.Hour
	if v := os.Getenv("OVERDUE_OPERATION_WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid OVERDUE_OPERATION_WORKER_INTERVAL %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("OVERDUE_OPERATION_WORKER_INTERVAL must be positive")
		}
		cfg.OverdueOperationWorkerInterval = d
	}

	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		for _, cidr := range strings.Split(v, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return Config{}, fmt.Errorf("invalid TRUSTED_PROXIES entry %q: %w", cidr, err)
			}
			cfg.TrustedProxies = append(cfg.TrustedProxies, cidr)
		}
	}

	cfg.TariffCacheTTL = 5 * time.Minute
	if v := os.Getenv("TARIFF_CACHE_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid TARIFF_CACHE_TTL %q: %w", v, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("TARIFF_CACHE_TTL must be positive")
		}
		cfg.TariffCacheTTL = d
	}

	return cfg, nil
}

func parseFloatEnv(key string, defaultValue float64) (float64, error) {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}
