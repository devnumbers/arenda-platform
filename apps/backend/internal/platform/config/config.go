package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                string
	HTTPAddr              string
	LogLevel              string
	LogLevelValue         slog.Level
	LogFormat             string
	DatabaseURL           string
	MigrationsDir         string
	CookieSecure          bool
	SMSSender             string
	PaymentProvider       string
	AppBaseURL            string
	TKassaTerminalKey     string
	TKassaPassword        string
	TKassaBaseURL         string
	TKassaTimeout         time.Duration
	EncryptionKey         string
	BillingWorkerInterval time.Duration
	LogSuccessfulRequests bool
	TariffCacheTTL        time.Duration
	RateLimit             RateLimit
	DBPool                DBPoolConfig
}

// RateLimit holds per-key rate-limiting configuration.
type RateLimit struct {
	IPRPS               float64
	IPBurst             int
	PhoneSendPerHour    int
	PhoneVerifyPer15Min int
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
		SMSSender:         os.Getenv("SMS_SENDER"),
		PaymentProvider:   os.Getenv("PAYMENT_PROVIDER"),
		AppBaseURL:        os.Getenv("APP_BASE_URL"),
		TKassaTerminalKey: os.Getenv("T_KASSA_TERMINAL_KEY"),
		TKassaPassword:    os.Getenv("T_KASSA_PASSWORD"),
		TKassaBaseURL:     os.Getenv("T_KASSA_BASE_URL"),
		EncryptionKey:     os.Getenv("ENCRYPTION_KEY"),
	}

	if cfg.AppEnv == "" {
		return Config{}, fmt.Errorf("APP_ENV is required")
	}
	allowedEnvs := map[string]bool{"local": true, "dev": true, "staging": true, "production": true}
	if !allowedEnvs[cfg.AppEnv] {
		return Config{}, fmt.Errorf("invalid APP_ENV %q: must be one of local, dev, staging, production", cfg.AppEnv)
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
		IPRPS:               20,
		IPBurst:             40,
		PhoneSendPerHour:    5,
		PhoneVerifyPer15Min: 10,
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
	if v := os.Getenv("RATE_LIMIT_PHONE_SEND_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_PHONE_SEND_PER_HOUR %q: %w", v, err)
		}
		cfg.RateLimit.PhoneSendPerHour = n
	}
	if v := os.Getenv("RATE_LIMIT_PHONE_VERIFY_PER_15MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_PHONE_VERIFY_PER_15MIN %q: %w", v, err)
		}
		cfg.RateLimit.PhoneVerifyPer15Min = n
	}
	if cfg.RateLimit.IPRPS <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_IP_RPS must be positive")
	}
	if cfg.RateLimit.IPBurst <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_IP_BURST must be positive")
	}
	if cfg.RateLimit.PhoneSendPerHour <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_PHONE_SEND_PER_HOUR must be positive")
	}
	if cfg.RateLimit.PhoneVerifyPer15Min <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT_PHONE_VERIFY_PER_15MIN must be positive")
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

	allowedSenders := map[string]bool{"": true, "fake": true}
	if !allowedSenders[cfg.SMSSender] {
		return Config{}, fmt.Errorf("invalid SMS_SENDER %q: must be empty or fake", cfg.SMSSender)
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
	}

	if cfg.AppEnv != "local" && cfg.EncryptionKey == "" {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY is required for APP_ENV=%s", cfg.AppEnv)
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
