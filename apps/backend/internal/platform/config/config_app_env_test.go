package config

import (
	"strings"
	"testing"
)

const (
	// Prefix of the Load error of the profiles that demand an explicit
	// PAYMENT_PROVIDER.
	errPaymentProviderRequired = "PAYMENT_PROVIDER is required"

	// T-Kassa test API accepted wherever tkassa is legal, stage included.
	tkassaSandboxBaseURL = "https://rest-api-test.tinkoff.ru/v2/"
)

// setRequiredDevEnv builds the dev profile on top of the local fixture: dev
// keeps the relaxed defaults (fake providers, optional T-Kassa base URL) but
// already requires an explicit payment provider and the encryption key.
func setRequiredDevEnv(t *testing.T) {
	t.Helper()
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", envDev)
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_PROVIDER", providerFake)
}

// setRequiredStageEnv builds the stage profile on top of the local fixture.
// Stage validates strictly like production; the fixture deliberately keeps
// the http APP_BASE_URL of the local one — only the tkassa rows override it,
// because fake payments and email do not carry the https requirement.
func setRequiredStageEnv(t *testing.T) {
	t.Helper()
	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", envStage)
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_PROVIDER", providerFake)
}

// requireStrictTKassa switches a fixture to tkassa payments outside the
// relaxed local/dev pair: both base URLs must be https, and the T-Kassa
// sandbox host stays legal alongside the production one.
func requireStrictTKassa(t *testing.T, tkassaBaseURL string) {
	t.Helper()
	t.Setenv("PAYMENT_PROVIDER", providerTKassa)
	t.Setenv("APP_BASE_URL", "https://stage.example.com")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_BASE_URL", tkassaBaseURL)
}

// requireTKassaCreds satisfies the provider credentials every tkassa row
// needs regardless of the environment profile.
func requireTKassaCreds(t *testing.T) {
	t.Helper()
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
}

func TestAppEnvVocabulary(t *testing.T) {
	setRequiredLocalEnv(t)

	t.Setenv("APP_ENV", "bogus")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "invalid APP_ENV") {
		t.Fatalf("expected invalid APP_ENV error, got: %v", err)
	}

	// The former staging spelling is retired: stage is the only stand
	// environment name, matching the GitHub Environment and compose files.
	t.Setenv("APP_ENV", "staging")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "invalid APP_ENV") {
		t.Fatalf("expected retired staging spelling to be rejected, got: %v", err)
	}

	t.Setenv("APP_ENV", envStage)
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("PAYMENT_PROVIDER", providerFake)
	if _, err := Load(); err != nil {
		t.Fatalf("expected stage to be a valid APP_ENV, got: %v", err)
	}
}

func TestPaymentProviderEnvMatrix(t *testing.T) {
	tests := []struct {
		name         string
		envSetup     func(t *testing.T)
		provider     string
		wantErr      string
		wantProvider string
	}{
		{
			name:         "local defaults to fake",
			envSetup:     setRequiredLocalEnv,
			provider:     "",
			wantProvider: providerFake,
		},
		{
			name:         "local allows fake",
			envSetup:     setRequiredLocalEnv,
			provider:     providerFake,
			wantProvider: providerFake,
		},
		{
			name: "local allows tkassa",
			envSetup: func(t *testing.T) {
				t.Helper()
				setRequiredLocalEnv(t)
				requireTKassaCreds(t)
			},
			provider:     providerTKassa,
			wantProvider: providerTKassa,
		},
		{
			name:     "dev requires explicit provider",
			envSetup: setRequiredDevEnv,
			provider: "",
			wantErr:  errPaymentProviderRequired,
		},
		{
			name:         "dev allows fake",
			envSetup:     setRequiredDevEnv,
			provider:     providerFake,
			wantProvider: providerFake,
		},
		{
			name: "dev allows tkassa",
			envSetup: func(t *testing.T) {
				t.Helper()
				setRequiredDevEnv(t)
				requireTKassaCreds(t)
			},
			provider:     providerTKassa,
			wantProvider: providerTKassa,
		},
		{
			name:     "stage requires explicit provider",
			envSetup: setRequiredStageEnv,
			provider: "",
			wantErr:  errPaymentProviderRequired,
		},
		{
			name:         "stage allows fake",
			envSetup:     setRequiredStageEnv,
			provider:     providerFake,
			wantProvider: providerFake,
		},
		{
			name: "stage allows tkassa with sandbox base URL",
			envSetup: func(t *testing.T) {
				t.Helper()
				setRequiredStageEnv(t)
				requireStrictTKassa(t, tkassaSandboxBaseURL)
			},
			provider:     providerTKassa,
			wantProvider: providerTKassa,
		},
		{
			name:     "production requires explicit provider",
			envSetup: setRequiredProductionEnv,
			provider: "",
			wantErr:  errPaymentProviderRequired,
		},
		{
			name:     "production rejects fake",
			envSetup: setRequiredProductionEnv,
			provider: providerFake,
			wantErr:  "PAYMENT_PROVIDER=fake is not allowed",
		},
		{
			name:         "production allows tkassa",
			envSetup:     setRequiredProductionEnv,
			provider:     providerTKassa,
			wantProvider: providerTKassa,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.envSetup(t)
			t.Setenv("PAYMENT_PROVIDER", tt.provider)

			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if cfg.PaymentProvider != tt.wantProvider {
				t.Fatalf("expected payment provider %q, got %q", tt.wantProvider, cfg.PaymentProvider)
			}
		})
	}
}

func TestEmailSenderEnvMatrix(t *testing.T) {
	tests := []struct {
		name     string
		envSetup func(t *testing.T)
		sender   string
		wantErr  string
		wantFake bool
	}{
		{
			name:     "local defaults to fake",
			envSetup: setRequiredLocalEnv,
			sender:   "",
			wantFake: true,
		},
		{
			name:     "local allows fake",
			envSetup: setRequiredLocalEnv,
			sender:   providerFake,
			wantFake: true,
		},
		{
			name:     "dev defaults to fake",
			envSetup: setRequiredDevEnv,
			sender:   "",
			wantFake: true,
		},
		{
			name:     "dev allows fake",
			envSetup: setRequiredDevEnv,
			sender:   providerFake,
			wantFake: true,
		},
		{
			name:     "stage requires explicit sender",
			envSetup: setRequiredStageEnv,
			sender:   "",
			wantErr:  "EMAIL_SENDER is required",
		},
		{
			name:     "stage allows fake",
			envSetup: setRequiredStageEnv,
			sender:   providerFake,
			wantFake: true,
		},
		{
			name:     "production requires explicit sender",
			envSetup: setRequiredProductionEnv,
			sender:   "",
			wantErr:  "EMAIL_SENDER is required",
		},
		{
			name:     "production rejects fake",
			envSetup: setRequiredProductionEnv,
			sender:   providerFake,
			wantErr:  "EMAIL_SENDER=fake is not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.envSetup(t)
			t.Setenv("EMAIL_SENDER", tt.sender)

			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}
			if tt.wantFake && cfg.EmailSender != providerFake {
				t.Fatalf("expected email sender fake, got %q", cfg.EmailSender)
			}
		})
	}
}

// The stage strictness pair below mirrors production on every check except
// the two fake-provider exceptions covered by the matrices.

func TestStageRequiresEncryptionKey(t *testing.T) {
	setRequiredStageEnv(t)
	t.Setenv("ENCRYPTION_KEY", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ENCRYPTION_KEY is required") {
		t.Fatalf("expected ENCRYPTION_KEY requirement on stage, got: %v", err)
	}
}

func TestStageRejectsExplicitInsecureCookie(t *testing.T) {
	setRequiredStageEnv(t)
	t.Setenv("COOKIE_SECURE", "false")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE=false is not allowed") {
		t.Fatalf("expected insecure cookie rejection on stage, got: %v", err)
	}
}

func TestStageRequiresAbsoluteSMTPTemplatesDir(t *testing.T) {
	setRequiredStageEnv(t)
	t.Setenv("EMAIL_SENDER", senderSMTP)
	t.Setenv("EMAIL_TEMPLATES_DIR", "apps/backend/templates/email")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("expected relative templates dir rejection on stage, got: %v", err)
	}
}

func TestStageRejectsHTTPAppBaseURLForTKassa(t *testing.T) {
	setRequiredStageEnv(t)
	requireStrictTKassa(t, tkassaSandboxBaseURL)
	t.Setenv("APP_BASE_URL", "http://stage.example.com")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected http APP_BASE_URL rejection on stage, got: %v", err)
	}
}

func TestStageRejectsUnknownTKassaBaseURLHost(t *testing.T) {
	setRequiredStageEnv(t)
	requireStrictTKassa(t, tkassaSandboxBaseURL)
	t.Setenv("T_KASSA_BASE_URL", "https://evil.example.com/v2/")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "securepay.tinkoff.ru") {
		t.Fatalf("expected unknown tkassa host rejection on stage, got: %v", err)
	}
}

func TestStageDefaultsToJSONLogs(t *testing.T) {
	setRequiredStageEnv(t)
	t.Setenv("LOG_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LogFormat != logFormatJSON {
		t.Fatalf("expected json logs on stage, got %q", cfg.LogFormat)
	}
}

func TestStageDefaultsToSecureCookies(t *testing.T) {
	setRequiredStageEnv(t)
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.CookieSecure {
		t.Fatal("expected secure cookies on stage")
	}
}
