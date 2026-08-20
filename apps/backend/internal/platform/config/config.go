// Package config loads and validates backend configuration from environment variables across APP_ENV profiles.
package config

import (
	"cmp"
	"errors"
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

// APP_ENV values: local and dev keep relaxed defaults (fake providers, optional
// T-Kassa base URL, pretty logs); staging and production validate fully.
const (
	envLocal = "local"
	envDev   = "dev"
)

// URL schemes accepted by outbound base-URL validation.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// providerFake is the shared no-integration value accepted by EMAIL_SENDER,
// PAYMENT_PROVIDER, and PHOTO_STORAGE_PROVIDER; senderSMTP is the real
// EMAIL_SENDER backend.
const (
	providerFake = "fake"
	senderSMTP   = "smtp"
)

type Config struct {
	AppEnv                              string
	AppVersion                          string
	HTTPAddr                            string
	LogLevel                            string
	LogLevelValue                       slog.Level
	LogFormat                           string
	DatabaseURL                         string
	MigrationsDir                       string
	AutoMigrate                         bool
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
	IdentityCleanerInterval             time.Duration
	IdentityCleanerRetention            time.Duration
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
	VAPIDPublicKey                      string
	// VAPIDPrivateKey and VAPIDSubject are consumed by the Web Push sender
	// (RFC 8292). They are optional: when VAPIDPublicKey is unset, push
	// delivery is disabled and the reminder worker runs email-only.
	VAPIDPrivateKey string
	VAPIDSubject    string
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

// Load reads the environment and validates every section in a fixed order, so
// the first failing section is deterministic. Each load* method owns one
// section and mutates the config in place; section bodies were extracted
// verbatim from the original monolithic Load (ticket #336, gocyclo gate).
func Load() (Config, error) {
	cfg := Config{
		AppEnv:            os.Getenv("APP_ENV"),
		AppVersion:        cmp.Or(os.Getenv("APP_VERSION"), "dev"),
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
		VAPIDPublicKey:   os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:  os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:     os.Getenv("VAPID_SUBJECT"),
	}

	for _, load := range []func() error{
		cfg.loadObservability,
		cfg.loadAppEnv,
		cfg.loadDaData,
		cfg.loadHTTPAddr,
		cfg.loadLogging,
		cfg.loadServer,
		cfg.loadRateLimit,
		cfg.loadDatabase,
		cfg.loadEmail,
		cfg.loadPaymentProvider,
		cfg.loadPhotoStorage,
		cfg.loadSchedulerIntervals,
		cfg.loadTrustedProxies,
		cfg.loadTariffCacheTTL,
	} {
		if err := load(); err != nil {
			return Config{}, err
		}
	}

	// VAPID keys are optional: the public key is served to the frontend when
	// set; the private key and subject are consumed by the Web Push sender
	// (RFC 8292). When the public key is unset, push delivery is disabled and
	// the reminder worker runs email-only.
	return cfg, nil
}

func (c *Config) loadObservability() error {
	sampler, err := parseFloatEnv("OTEL_TRACES_SAMPLER_ARG", 1.0)
	if err != nil {
		return fmt.Errorf("invalid OTEL_TRACES_SAMPLER_ARG %q: %w", os.Getenv("OTEL_TRACES_SAMPLER_ARG"), err)
	}
	c.OTelTraceSampler = sampler

	if c.OTelEnabled && c.OTelOTLPEndpoint == "" {
		return errors.New(
			"OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_TRACES_EXPORTER or " +
				"OTEL_METRICS_EXPORTER is set to a value other than 'none'")
	}
	return nil
}

func (c *Config) loadAppEnv() error {
	if c.AppEnv == "" {
		return errors.New("APP_ENV is required")
	}
	allowedEnvs := map[string]bool{envLocal: true, envDev: true, "staging": true, "production": true}
	if !allowedEnvs[c.AppEnv] {
		return fmt.Errorf("invalid APP_ENV %q: must be one of local, dev, staging, production", c.AppEnv)
	}
	return nil
}

func (c *Config) loadDaData() error {
	if c.DaDataBaseURL == "" {
		c.DaDataBaseURL = "https://suggestions.dadata.ru/suggestions/api/4_1/rs/suggest/address"
	}
	if dadataURL, err := url.Parse(c.DaDataBaseURL); err != nil {
		return fmt.Errorf("invalid DADATA_BASE_URL %q: %w", c.DaDataBaseURL, err)
	} else if dadataURL.Scheme != schemeHTTP && dadataURL.Scheme != schemeHTTPS {
		return fmt.Errorf("invalid DADATA_BASE_URL %q: scheme must be http or https", c.DaDataBaseURL)
	}

	c.DaDataTimeout = 10 * time.Second
	if v := os.Getenv("DADATA_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid DADATA_TIMEOUT %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("DADATA_TIMEOUT must be positive")
		}
		c.DaDataTimeout = d
	}

	if c.DaDataAPIKey == "" {
		return errors.New("DADATA_API_KEY is required")
	}
	return nil
}

func (c *Config) loadHTTPAddr() error {
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}
	return nil
}

func (c *Config) loadLogging() error {
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if err := c.LogLevelValue.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return fmt.Errorf("invalid LOG_LEVEL %q: %w", c.LogLevel, err)
	}

	if c.LogFormat == "" {
		switch c.AppEnv {
		case envLocal, envDev:
			c.LogFormat = "pretty"
		default:
			c.LogFormat = "json"
		}
	}
	allowedFormats := map[string]bool{"json": true, "pretty": true}
	if !allowedFormats[c.LogFormat] {
		return fmt.Errorf("invalid LOG_FORMAT %q: must be json or pretty", c.LogFormat)
	}

	c.LogSuccessfulRequests = true
	if v := os.Getenv("LOG_SUCCESSFUL_REQUESTS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid LOG_SUCCESSFUL_REQUESTS %q: %w", v, err)
		}
		c.LogSuccessfulRequests = b
	}
	return nil
}

func (c *Config) loadServer() error {
	c.AutoMigrate = true
	if v := os.Getenv("AUTO_MIGRATE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid AUTO_MIGRATE %q: %w", v, err)
		}
		c.AutoMigrate = b
	}

	cookieSecure := os.Getenv("COOKIE_SECURE")
	cookieSecureExplicit := false
	if cookieSecure != "" {
		v, err := strconv.ParseBool(cookieSecure)
		if err != nil {
			return fmt.Errorf("invalid COOKIE_SECURE %q: %w", cookieSecure, err)
		}
		c.CookieSecure = v
		cookieSecureExplicit = true
	}
	if !cookieSecureExplicit {
		switch c.AppEnv {
		case envLocal:
			c.CookieSecure = false
		default:
			c.CookieSecure = true
		}
	}
	if !c.CookieSecure && c.AppEnv != envLocal && cookieSecureExplicit {
		return fmt.Errorf("COOKIE_SECURE=false is not allowed for APP_ENV=%s", c.AppEnv)
	}
	return nil
}

func (c *Config) loadRateLimit() error {
	c.RateLimit = defaultRateLimit()
	if err := c.overrideRateLimitIP(); err != nil {
		return err
	}
	if err := c.overrideRateLimitEmail(); err != nil {
		return err
	}
	if err := c.overrideRateLimitPhoneChange(); err != nil {
		return err
	}
	return c.validateRateLimit()
}

// defaultRateLimit returns the built-in rate-limit defaults every env
// override is applied on top of.
func defaultRateLimit() RateLimit {
	return RateLimit{
		IPRPS:                     20,
		IPBurst:                   40,
		EmailSendPerHour:          60,
		EmailVerifyPer15Min:       30,
		PhoneChangeSendPerHour:    5,
		PhoneChangeVerifyPer15Min: 10,
	}
}

// overrideRateLimitIP applies the per-IP request-rate overrides.
func (c *Config) overrideRateLimitIP() error {
	if v := os.Getenv("RATE_LIMIT_IP_RPS"); v != "" {
		rps, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_IP_RPS %q: %w", v, err)
		}
		c.RateLimit.IPRPS = rps
	}
	if v := os.Getenv("RATE_LIMIT_IP_BURST"); v != "" {
		burst, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_IP_BURST %q: %w", v, err)
		}
		c.RateLimit.IPBurst = burst
	}
	return nil
}

// overrideRateLimitEmail applies the email send/verify overrides.
func (c *Config) overrideRateLimitEmail() error {
	if v := os.Getenv("RATE_LIMIT_EMAIL_SEND_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_EMAIL_SEND_PER_HOUR %q: %w", v, err)
		}
		c.RateLimit.EmailSendPerHour = n
	}
	if v := os.Getenv("RATE_LIMIT_EMAIL_VERIFY_PER_15MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_EMAIL_VERIFY_PER_15MIN %q: %w", v, err)
		}
		c.RateLimit.EmailVerifyPer15Min = n
	}
	return nil
}

// overrideRateLimitPhoneChange applies the phone-change send/verify overrides,
// keeping the built-in defaults when a value arrives unset or non-positive.
func (c *Config) overrideRateLimitPhoneChange() error {
	if v := os.Getenv("RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR %q: %w", v, err)
		}
		c.RateLimit.PhoneChangeSendPerHour = n
	} else if c.RateLimit.PhoneChangeSendPerHour <= 0 {
		c.RateLimit.PhoneChangeSendPerHour = 5
	}
	if v := os.Getenv("RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN %q: %w", v, err)
		}
		c.RateLimit.PhoneChangeVerifyPer15Min = n
	} else if c.RateLimit.PhoneChangeVerifyPer15Min <= 0 {
		c.RateLimit.PhoneChangeVerifyPer15Min = 10
	}
	return nil
}

// validateRateLimit rejects non-positive limits in the fixed section order.
func (c *Config) validateRateLimit() error {
	if c.RateLimit.IPRPS <= 0 {
		return errors.New("RATE_LIMIT_IP_RPS must be positive")
	}
	if c.RateLimit.IPBurst <= 0 {
		return errors.New("RATE_LIMIT_IP_BURST must be positive")
	}
	if c.RateLimit.EmailSendPerHour <= 0 {
		return errors.New("RATE_LIMIT_EMAIL_SEND_PER_HOUR must be positive")
	}
	if c.RateLimit.EmailVerifyPer15Min <= 0 {
		return errors.New("RATE_LIMIT_EMAIL_VERIFY_PER_15MIN must be positive")
	}
	if c.RateLimit.PhoneChangeSendPerHour <= 0 {
		return errors.New("RATE_LIMIT_PHONE_CHANGE_SEND_PER_HOUR must be positive")
	}
	if c.RateLimit.PhoneChangeVerifyPer15Min <= 0 {
		return errors.New("RATE_LIMIT_PHONE_CHANGE_VERIFY_PER_15MIN must be positive")
	}
	return nil
}

func (c *Config) loadDatabase() error {
	if err := c.validateDatabaseURL(); err != nil {
		return err
	}
	c.DBPool = defaultDBPool()
	if err := c.overrideDBPoolSizes(); err != nil {
		return err
	}
	if err := c.overrideDBPoolTimeouts(); err != nil {
		return err
	}
	if err := c.overrideDBPoolHealthCheck(); err != nil {
		return err
	}
	return c.validateDBPool()
}

// validateDatabaseURL checks the required connection URL and migrations dir.
func (c *Config) validateDatabaseURL() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if !strings.HasPrefix(c.DatabaseURL, "postgres://") && !strings.HasPrefix(c.DatabaseURL, "postgresql://") {
		return errors.New("invalid DATABASE_URL: must start with postgres:// or postgresql://")
	}
	if c.MigrationsDir == "" {
		return errors.New("MIGRATIONS_DIR is required")
	}
	return nil
}

// defaultDBPool returns the built-in pool settings every env override is
// applied on top of.
func defaultDBPool() DBPoolConfig {
	return DBPoolConfig{
		MaxConns:                        64,
		MinConns:                        16,
		MaxConnLifetime:                 30 * time.Minute,
		MaxConnIdleTime:                 5 * time.Minute,
		HealthCheckPeriod:               30 * time.Second,
		StatementTimeout:                30 * time.Second,
		IdleInTransactionSessionTimeout: 60 * time.Second,
	}
}

// overrideDBPoolSizes applies the connection-count overrides.
func (c *Config) overrideDBPoolSizes() error {
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid DB_MAX_CONNS %q: %w", v, err)
		}
		c.DBPool.MaxConns = int32(n)
	}
	if v := os.Getenv("DB_MIN_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid DB_MIN_CONNS %q: %w", v, err)
		}
		c.DBPool.MinConns = int32(n)
	}
	return nil
}

// overrideDBPoolTimeouts applies the statement and lifetime timeout
// overrides; zero or negative values stay allowed here.
func (c *Config) overrideDBPoolTimeouts() error {
	if err := overrideDurationEnv(&c.DBPool.MaxConnLifetime, "DB_MAX_CONN_LIFETIME"); err != nil {
		return err
	}
	if err := overrideDurationEnv(&c.DBPool.MaxConnIdleTime, "DB_MAX_CONN_IDLE_TIME"); err != nil {
		return err
	}
	if err := overrideDurationEnv(&c.DBPool.StatementTimeout, "DB_STATEMENT_TIMEOUT"); err != nil {
		return err
	}
	return overrideDurationEnv(&c.DBPool.IdleInTransactionSessionTimeout, "DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT")
}

// overrideDBPoolHealthCheck applies the health-check period override, which
// must stay positive.
func (c *Config) overrideDBPoolHealthCheck() error {
	if v := os.Getenv("DB_HEALTH_CHECK_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid DB_HEALTH_CHECK_PERIOD %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("DB_HEALTH_CHECK_PERIOD must be positive")
		}
		c.DBPool.HealthCheckPeriod = d
	}
	return nil
}

// validateDBPool rejects pool sizes that cannot serve traffic.
func (c *Config) validateDBPool() error {
	if c.DBPool.MaxConns <= 0 {
		return errors.New("DB_MAX_CONNS must be positive")
	}
	if c.DBPool.MinConns < 0 {
		return errors.New("DB_MIN_CONNS must be non-negative")
	}
	if c.DBPool.MinConns > c.DBPool.MaxConns {
		return errors.New("DB_MIN_CONNS must not exceed DB_MAX_CONNS")
	}
	return nil
}

func (c *Config) loadEmail() error {
	if err := c.validateEmailSender(); err != nil {
		return err
	}
	if err := c.loadSMTPSettings(); err != nil {
		return err
	}
	return nil
}

// validateEmailSender pins EMAIL_SENDER to the allowed values and the
// environment profile: empty defaults to fake in local/dev only and fake is
// rejected in every stricter environment.
func (c *Config) validateEmailSender() error {
	allowedEmailSenders := map[string]bool{"": true, providerFake: true, senderSMTP: true}
	if !allowedEmailSenders[c.EmailSender] {
		return fmt.Errorf("invalid EMAIL_SENDER %q: must be empty, fake, or smtp", c.EmailSender)
	}
	if c.EmailSender == "" {
		if c.AppEnv != envLocal && c.AppEnv != envDev {
			return fmt.Errorf("EMAIL_SENDER is required for APP_ENV=%s", c.AppEnv)
		}
		c.EmailSender = providerFake
	}
	if c.EmailSender == providerFake && c.AppEnv != envLocal && c.AppEnv != envDev {
		return fmt.Errorf("EMAIL_SENDER=fake is not allowed for APP_ENV=%s", c.AppEnv)
	}
	return nil
}

// loadSMTPSettings resolves the templates dir and validates the SMTP_* fields
// required when EMAIL_SENDER=smtp.
func (c *Config) loadSMTPSettings() error {
	if c.EmailTemplatesDir == "" {
		c.EmailTemplatesDir = "apps/backend/templates/email"
	}
	if c.EmailSender == senderSMTP && c.AppEnv != envLocal && c.AppEnv != envDev {
		if !filepath.IsAbs(c.EmailTemplatesDir) {
			return fmt.Errorf("EMAIL_TEMPLATES_DIR must be an absolute path in %s environment", c.AppEnv)
		}
	}
	if c.EmailSender != senderSMTP {
		return nil
	}
	if err := c.validateSMTPFields(); err != nil {
		return err
	}
	return overridePositiveDurationEnv(&c.SMTPTimeout, "SMTP_TIMEOUT")
}

// validateSMTPFields checks the required connection and envelope fields of
// the smtp sender.
func (c *Config) validateSMTPFields() error {
	if c.SMTPHost == "" {
		return errors.New("SMTP_HOST is required when EMAIL_SENDER=smtp")
	}
	if c.SMTPPort == "" {
		return errors.New("SMTP_PORT is required when EMAIL_SENDER=smtp")
	}
	if c.SMTPFrom == "" {
		return errors.New("SMTP_FROM is required when EMAIL_SENDER=smtp")
	}
	return nil
}

func (c *Config) loadPaymentProvider() error {
	if c.PaymentProvider == "" {
		if c.AppEnv != envLocal {
			return fmt.Errorf("PAYMENT_PROVIDER is required for APP_ENV=%s", c.AppEnv)
		}
		c.PaymentProvider = providerFake
	}
	allowedPaymentProviders := map[string]bool{providerFake: true, "tkassa": true}
	if !allowedPaymentProviders[c.PaymentProvider] {
		return fmt.Errorf("invalid PAYMENT_PROVIDER %q: must be fake or tkassa", c.PaymentProvider)
	}
	if c.AppEnv != envLocal && c.AppEnv != envDev && c.PaymentProvider == providerFake {
		return fmt.Errorf("PAYMENT_PROVIDER=fake is not allowed for APP_ENV=%s", c.AppEnv)
	}
	if c.PaymentProvider == providerFake {
		if err := c.loadFakeProviderBaseURL(); err != nil {
			return err
		}
	}
	if c.PaymentProvider == "tkassa" {
		if err := c.loadTKassa(); err != nil {
			return err
		}
	}

	if c.AppEnv != envLocal && c.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required for APP_ENV=%s", c.AppEnv)
	}
	return nil
}

func (c *Config) loadFakeProviderBaseURL() error {
	if c.AppBaseURL == "" {
		return errors.New("APP_BASE_URL is required when PAYMENT_PROVIDER=fake")
	}
	u, err := url.Parse(c.AppBaseURL)
	if err != nil {
		return fmt.Errorf("invalid APP_BASE_URL %q: %w", c.AppBaseURL, err)
	}
	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
		return fmt.Errorf("invalid APP_BASE_URL %q: scheme must be http or https", c.AppBaseURL)
	}
	return nil
}

func (c *Config) loadTKassa() error {
	if err := c.validateTKassaCredentials(); err != nil {
		return err
	}
	if err := c.validateTKassaAppBaseURL(); err != nil {
		return err
	}
	if err := c.loadTKassaBaseURL(); err != nil {
		return err
	}
	c.TKassaTimeout = 30 * time.Second
	if err := overridePositiveDurationEnv(&c.TKassaTimeout, "T_KASSA_TIMEOUT"); err != nil {
		return err
	}
	if err := c.overrideTKassaMaxRetries(); err != nil {
		return err
	}
	c.TKassaRetryBaseDelay = 500 * time.Millisecond
	if err := overridePositiveDurationEnv(&c.TKassaRetryBaseDelay, "T_KASSA_RETRY_BASE_DELAY"); err != nil {
		return err
	}
	c.TKassaRetryMaxDelay = 5 * time.Second
	return overridePositiveDurationEnv(&c.TKassaRetryMaxDelay, "T_KASSA_RETRY_MAX_DELAY")
}

// validateTKassaCredentials checks the terminal credentials and that an app
// base URL is present to redirect payments back to.
func (c *Config) validateTKassaCredentials() error {
	if c.TKassaTerminalKey == "" {
		return errors.New("T_KASSA_TERMINAL_KEY is required when PAYMENT_PROVIDER=tkassa")
	}
	if c.TKassaPassword == "" {
		return errors.New("T_KASSA_PASSWORD is required when PAYMENT_PROVIDER=tkassa")
	}
	if c.AppBaseURL == "" {
		return errors.New("APP_BASE_URL is required when PAYMENT_PROVIDER=tkassa")
	}
	return nil
}

// validateTKassaAppBaseURL checks the app base URL scheme; non-local/dev
// environments must run behind https.
func (c *Config) validateTKassaAppBaseURL() error {
	u, err := url.Parse(c.AppBaseURL)
	if err != nil {
		return fmt.Errorf("invalid APP_BASE_URL %q: %w", c.AppBaseURL, err)
	}
	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
		return fmt.Errorf("invalid APP_BASE_URL %q: scheme must be http or https", c.AppBaseURL)
	}
	if c.AppEnv != envLocal && c.AppEnv != envDev && u.Scheme != schemeHTTPS {
		return fmt.Errorf("invalid APP_BASE_URL %q: non-local/dev environments must use https", c.AppBaseURL)
	}
	return nil
}

// overrideTKassaMaxRetries applies the retry-count override on top of the
// built-in default.
func (c *Config) overrideTKassaMaxRetries() error {
	c.TKassaMaxRetries = 3
	v := os.Getenv("T_KASSA_MAX_RETRIES")
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid T_KASSA_MAX_RETRIES %q: %w", v, err)
	}
	if n < 0 {
		return errors.New("T_KASSA_MAX_RETRIES must be non-negative")
	}
	c.TKassaMaxRetries = n
	return nil
}

func (c *Config) loadTKassaBaseURL() error {
	if c.TKassaBaseURL == "" {
		if c.AppEnv != envLocal && c.AppEnv != envDev {
			return fmt.Errorf("T_KASSA_BASE_URL is required when PAYMENT_PROVIDER=tkassa for APP_ENV=%s", c.AppEnv)
		}
		return nil
	}
	tku, err := url.Parse(c.TKassaBaseURL)
	if err != nil {
		return fmt.Errorf("invalid T_KASSA_BASE_URL %q: %w", c.TKassaBaseURL, err)
	}
	if tku.Scheme != schemeHTTP && tku.Scheme != schemeHTTPS {
		return fmt.Errorf("invalid T_KASSA_BASE_URL %q: scheme must be http or https", c.TKassaBaseURL)
	}
	if c.AppEnv != envLocal && c.AppEnv != envDev {
		if tku.Scheme != "https" {
			return fmt.Errorf("invalid T_KASSA_BASE_URL %q: non-local/dev environments must use https", c.TKassaBaseURL)
		}
		host := strings.ToLower(tku.Hostname())
		if host != "securepay.tinkoff.ru" && host != "rest-api-test.tinkoff.ru" {
			return fmt.Errorf(
				"invalid T_KASSA_BASE_URL %q: production T-Kassa base URL must be "+
					"https://securepay.tinkoff.ru/v2/ or https://rest-api-test.tinkoff.ru/v2/",
				c.TKassaBaseURL)
		}
		if strings.TrimSuffix(tku.Path, "/") != "/v2" {
			return fmt.Errorf("invalid T_KASSA_BASE_URL %q: path must be /v2/", c.TKassaBaseURL)
		}
	}
	return nil
}

func (c *Config) loadPhotoStorage() error {
	c.PhotoStorageEndpoint = os.Getenv("REGRU_S3_ENDPOINT")
	c.PhotoStorageRegion = os.Getenv("REGRU_S3_REGION")
	c.PhotoStorageBucket = os.Getenv("REGRU_S3_BUCKET")
	c.PhotoStorageAccessKey = os.Getenv("REGRU_S3_ACCESS_KEY")
	c.PhotoStorageSecretKey = os.Getenv("REGRU_S3_SECRET_KEY")
	c.PhotoStoragePublicBaseURL = os.Getenv("REGRU_S3_PUBLIC_BASE_URL")
	c.PhotoStorageProvider = strings.ToLower(strings.TrimSpace(os.Getenv("PHOTO_STORAGE_PROVIDER")))

	c.PhotoStoragePathStyle = true
	if v := os.Getenv("REGRU_S3_PATH_STYLE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid REGRU_S3_PATH_STYLE %q: %w", v, err)
		}
		c.PhotoStoragePathStyle = b
	}

	s3Fields := []string{
		c.PhotoStorageEndpoint,
		c.PhotoStorageBucket,
		c.PhotoStorageAccessKey,
		c.PhotoStorageSecretKey,
		c.PhotoStoragePublicBaseURL,
	}
	s3Complete := !slices.Contains(s3Fields, "")
	if c.PhotoStorageProvider == "" {
		if c.AppEnv == envLocal && !s3Complete {
			c.PhotoStorageProvider = providerFake
		} else {
			c.PhotoStorageProvider = "s3"
		}
	}
	switch c.PhotoStorageProvider {
	case providerFake:
		c.PhotoStorageS3Enabled = false
		c.PhotoStoragePublicBaseURL = strings.TrimRight(c.AppBaseURL, "/") + "/uploads"
	case "s3":
		if !s3Complete {
			return fmt.Errorf(
				"REGRU_S3_ENDPOINT, REGRU_S3_BUCKET, REGRU_S3_ACCESS_KEY, REGRU_S3_SECRET_KEY "+
					"and REGRU_S3_PUBLIC_BASE_URL are required for PHOTO_STORAGE_PROVIDER=s3 "+
					"and APP_ENV=%s; set PHOTO_STORAGE_PROVIDER=fake only for temporary "+
					"launches without photo uploads", c.AppEnv)
		}
		c.PhotoStorageS3Enabled = true
	default:
		return fmt.Errorf("invalid PHOTO_STORAGE_PROVIDER %q: must be fake or s3", c.PhotoStorageProvider)
	}
	return nil
}

func (c *Config) loadSchedulerIntervals() error {
	c.BillingWorkerInterval = time.Hour
	if v := os.Getenv("BILLING_WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid BILLING_WORKER_INTERVAL %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("BILLING_WORKER_INTERVAL must be positive")
		}
		c.BillingWorkerInterval = d
	}

	c.PaymentReconciliationWorkerInterval = 5 * time.Minute
	if v := os.Getenv("PAYMENT_RECONCILIATION_WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid PAYMENT_RECONCILIATION_WORKER_INTERVAL %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("PAYMENT_RECONCILIATION_WORKER_INTERVAL must be positive")
		}
		c.PaymentReconciliationWorkerInterval = d
	}

	c.IdentityCleanerInterval = time.Hour
	if v := os.Getenv("IDENTITY_CLEANER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid IDENTITY_CLEANER_INTERVAL %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("IDENTITY_CLEANER_INTERVAL must be positive")
		}
		c.IdentityCleanerInterval = d
	}

	c.IdentityCleanerRetention = 7 * 24 * time.Hour
	if v := os.Getenv("IDENTITY_CLEANER_RETENTION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid IDENTITY_CLEANER_RETENTION %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("IDENTITY_CLEANER_RETENTION must be positive")
		}
		c.IdentityCleanerRetention = d
	}
	return nil
}

func (c *Config) loadTrustedProxies() error {
	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		for cidr := range strings.SplitSeq(v, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return fmt.Errorf("invalid TRUSTED_PROXIES entry %q: %w", cidr, err)
			}
			c.TrustedProxies = append(c.TrustedProxies, cidr)
		}
	}
	return nil
}

func (c *Config) loadTariffCacheTTL() error {
	c.TariffCacheTTL = 5 * time.Minute
	if v := os.Getenv("TARIFF_CACHE_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid TARIFF_CACHE_TTL %q: %w", v, err)
		}
		if d <= 0 {
			return errors.New("TARIFF_CACHE_TTL must be positive")
		}
		c.TariffCacheTTL = d
	}
	return nil
}

// overrideDurationEnv overrides dst from the KEY env var when set; zero and
// negative values stay allowed (callers that need positivity validate
// separately).
func overrideDurationEnv(dst *time.Duration, key string) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	*dst = d
	return nil
}

// overridePositiveDurationEnv overrides dst from the KEY env var when set and
// rejects zero or negative values.
func overridePositiveDurationEnv(dst *time.Duration, key string) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	if d <= 0 {
		return errors.New(key + " must be positive")
	}
	*dst = d
	return nil
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
