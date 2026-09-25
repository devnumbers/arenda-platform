package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/application"
)

// NewRecorder builds the real action journal recorder over the pool — the
// same composition the wire runs (карта #704, ADR 0061): both stores over
// one pool, the clock defaulting to the real one. Lives here, next to the
// stores it composes; the integration suites of the calling modules run
// their journals through it.
func NewRecorder(pool *pgxpool.Pool) application.Recorder {
	return application.NewService(NewEntryStore(pool), NewActorStore(pool), nil)
}
