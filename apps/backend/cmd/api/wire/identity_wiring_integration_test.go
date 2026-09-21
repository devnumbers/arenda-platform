//go:build integration

package wire

// Wiring guard for the identity assembly (testing-strategy, layer 3): the
// composition bug fixed in 84d14023 — EmailChange left out of the
// httpserver.Deps literal, so every /me/email/* route answered 500 and only
// live e2e caught it — is exactly the class this family closes. The test
// drives the real WireIdentity over the test pool and pins the assembly
// invariant: every service and adapter of the identity module comes out
// non-nil. Removing any wiring action inside WireIdentity turns it red.

import (
	"context"
	"log/slog"
	"testing"

	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/stretchr/testify/require"
)

func TestWireIdentityAssembly(t *testing.T) {
	t.Parallel()

	pool := testdb.Setup(t)
	logger := slog.New(slog.DiscardHandler)
	db := database.NewInstrumentedPool(pool, logger)

	encryptor, err := encryption.NewEncryptor("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	require.NoError(t, err)

	renderer, err := mailer.NewRenderer("../../../templates/email")
	require.NoError(t, err)

	cfg := &config.Config{
		EmailSender: providerFake,
		RateLimit: config.RateLimit{
			IPRPS:                     20,
			IPBurst:                   40,
			EmailSendPerHour:          60,
			EmailVerifyPer15Min:       30,
			PhoneChangeSendPerHour:    5,
			PhoneChangeVerifyPer15Min: 10,
			EmailChangeSendPerHour:    5,
		},
	}
	limits := WireRateLimiters(cfg)
	defer limits.Stop()

	identity, err := WireIdentity(
		context.Background(),
		platformDeps{
			Cfg:           cfg,
			DB:            db,
			Logger:        logger,
			Encryptor:     encryptor,
			Renderer:      renderer,
			AuditRecorder: auditapp.NewService(auditpg.NewWriter(db), clock.Real{}),
			Clock:         clock.Real{},
			UoW:           postgres.NewUoW(pool, logger),
		},
		events.NewInProcessDispatcher(),
		limits,
	)
	require.NoError(t, err)

	require.NotNil(t, identity.UserRepo)
	require.NotNil(t, identity.CodeRepo)
	require.NotNil(t, identity.AttemptRepo)
	require.NotNil(t, identity.SessionRepo)
	require.NotNil(t, identity.SessionService)
	require.NotNil(t, identity.SessionLoader)
	require.NotNil(t, identity.EventPublisher)
	require.NotNil(t, identity.Authentication)
	require.NotNil(t, identity.PhoneChange)
	// The exact field 84d14023 left out of the composition root.
	require.NotNil(t, identity.EmailChange)
	require.NotNil(t, identity.Profile)
	require.NotNil(t, identity.Logout)
	require.NotNil(t, identity.EmailMailer)
}
