//go:build integration

package wire

// Wiring guard for the access module's global revoke (testing-strategy,
// layer 3; issue #1310): the targetless «Отозвать доступ всем» gates on the
// injected RevokeAllScope probe — a forgotten injection would leave the
// service with a nil scope and every call wide open (the nil scope speaks
// owner for every actor). The test drives the real WireAccess over the test
// pool and reads the service's unexported `scope` field back through
// reflection, the main_wiring_test canon: removing the injection inside
// WireAccess turns this test red with the field's verdict (закомментировал —
// упал — вернул).

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/config"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/stretchr/testify/require"
)

func TestWireAccessRevokeAllScopeAssembly(t *testing.T) {
	t.Parallel()

	pool := testdb.Setup(t)
	logger := slog.New(slog.DiscardHandler)
	db := database.NewInstrumentedPool(pool, logger)

	encryptor, err := encryption.NewEncryptor("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	require.NoError(t, err)

	renderer, err := mailer.NewRenderer("../../../templates/email")
	require.NoError(t, err)

	// WireAccess reads only the billing limiter out of the billing module:
	// the two repos and the clock are the whole bridge.
	limiter := billingapp.NewSubscriptionPropertyLimiter(
		billingpg.NewSubscriptionRepository(db),
		billingpg.NewTariffRepository(db, 0, clock.Real{}),
		clock.Real{},
	)

	access, err := WireAccess(
		context.Background(),
		platformDeps{
			Cfg:             &config.Config{},
			DB:              db,
			Logger:          logger,
			Encryptor:       encryptor,
			Renderer:        renderer,
			AuditRecorder:   auditapp.NewService(auditpg.NewWriter(db), clock.Real{}),
			HistoryRecorder: historyapp.NewService(historypg.NewEntryStore(db), historypg.NewActorStore(db), clock.Real{}),
			Clock:           clock.Real{},
			Beginner:        platformpostgres.NewBeginner(pool, logger),
			UoW:             platformpostgres.NewUoW(pool, logger),
		},
		&Billing{Services: billingapp.Services{Limiter: limiter}},
		nil, // The mailer is unused at wiring time.
		events.NewInProcessDispatcher(),
	)
	require.NoError(t, err)
	require.NotNil(t, access)

	svc := access.ParticipantMutationSvc
	require.NotNil(t, svc)
	scopeField := reflect.ValueOf(svc).Elem().FieldByName("scope")
	require.True(t, scopeField.IsValid(),
		"ParticipantMutationService has no `scope` field — the wiring test follows a renamed injection")
	require.False(t, scopeField.IsNil(),
		"ParticipantMutationService lost its RevokeAllScope — the wiring in WireAccess is incomplete, the global revoke would run ungated")
	require.Equal(t, reflect.TypeFor[*accesspg.RevokeAllScope](), scopeField.Elem().Type(),
		"ParticipantMutationService holds something other than the postgres RevokeAllScope adapter")
}
