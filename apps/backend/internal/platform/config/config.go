package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	LogLevel      string
	LogLevelValue slog.Level
	LogFormat     string
	DatabaseURL   string
	MigrationsDir string
	CookieSecure  bool
	SMSSender     string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:        os.Getenv("APP_ENV"),
		HTTPAddr:      os.Getenv("HTTP_ADDR"),
		LogLevel:      os.Getenv("LOG_LEVEL"),
		LogFormat:     os.Getenv("LOG_FORMAT"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),
		SMSSender:     os.Getenv("SMS_SENDER"),
	}

	if cfg.AppEnv == "" {
		cfg.AppEnv = "local"
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

	cookieSecure := os.Getenv("COOKIE_SECURE")
	if cookieSecure != "" {
		v, err := strconv.ParseBool(cookieSecure)
		if err != nil {
			return Config{}, fmt.Errorf("invalid COOKIE_SECURE %q: %w", cookieSecure, err)
		}
		cfg.CookieSecure = v
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

	allowedSenders := map[string]bool{"": true, "fake": true}
	if !allowedSenders[cfg.SMSSender] {
		return Config{}, fmt.Errorf("invalid SMS_SENDER %q: must be empty or fake", cfg.SMSSender)
	}

	return cfg, nil
}
