package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// advisoryLockReleaseTimeout bounds the lock-release call. It runs on a context
// detached from the caller's cancellation (context.WithoutCancel) so shutdown
// cannot leak a held advisory lock, but it must not hang forever either.
const advisoryLockReleaseTimeout = 5 * time.Second

// runTickerLoop drives a periodic scheduler worker: one tick immediately at
// startup, then one tick per interval, until ctx is cancelled. A tick error is
// logged (with the worker name) and does not stop the loop — the next tick
// retries. It is the shared Run skeleton extracted from the billing and
// payment reconciliation workers (issue #337).
func runTickerLoop(ctx context.Context, worker string, interval time.Duration, logger *slog.Logger, tick func(context.Context) error) {
	logger.InfoContext(ctx, "worker started", "worker", worker, "interval", interval.String())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runTick := func() {
		if err := tick(ctx); err != nil {
			logger.ErrorContext(ctx, "worker tick failed", "worker", worker, "error", sanitize.Error(err))
		}
	}
	runTick()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runTick()
		}
	}
}

// withAdvisoryTickLock elects a single leader for the current tick using a
// PostgreSQL advisory lock and runs work only on the leader. When another
// instance already holds the lock, work is skipped without error (logged at
// info). The lock is released on a context detached from the caller's
// cancellation, so a cancelled tick still returns its connection and lock.
func withAdvisoryTickLock(
	ctx context.Context,
	pool *pgxpool.Pool,
	key int64,
	worker string,
	logger *slog.Logger,
	work func(context.Context) error,
) error {
	acquired, release, err := tryAcquireAdvisoryLock(ctx, pool, key, worker, logger)
	if err != nil {
		return err
	}
	if !acquired {
		logger.InfoContext(ctx, "worker tick skipped, another instance holds the lock", "worker", worker)
		return nil
	}
	defer releaseWithTimeout(ctx, release)

	return work(ctx)
}

// releaseWithTimeout releases the advisory lock on a context detached from the
// caller's cancellation with a bounded deadline, so a cancelled tick still
// returns its lock and connection without hanging shutdown.
func releaseWithTimeout(ctx context.Context, release func(context.Context)) {
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), advisoryLockReleaseTimeout)
	defer cancel()
	release(releaseCtx)
}

// tryAcquireAdvisoryLock takes a session-level advisory lock on a dedicated
// pool connection. The returned release function releases the lock and returns
// the connection to the pool; it must be called exactly once when the caller no
// longer needs the lock.
func tryAcquireAdvisoryLock(
	ctx context.Context,
	pool *pgxpool.Pool,
	key int64,
	worker string,
	logger *slog.Logger,
) (acquired bool, release func(context.Context), err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquire db connection for lock: %w", err)
	}

	release = func(releaseCtx context.Context) {
		if _, unlockErr := conn.Exec(releaseCtx, "SELECT pg_advisory_unlock($1)", key); unlockErr != nil {
			logger.ErrorContext(releaseCtx, "worker failed to release advisory lock", "worker", worker, "error", sanitize.Error(unlockErr))
		}
		conn.Release()
	}

	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		releaseWithTimeout(ctx, release)
		return false, nil, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !acquired {
		releaseWithTimeout(ctx, release)
		return false, nil, nil
	}
	return true, release, nil
}
