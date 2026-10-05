// Package idempotency is the server-side Idempotency-Key storage for
// creation endpoints (тикет Т3 карты #1112, ресерч #1114): the last line of
// defense against duplicate creations — a replayed request with the same key
// returns the stored result of the first attempt (the Stripe Idempotent
// Requests pattern; there is no HTTP standard for the header — the IETF draft
// is expired — so the de-facto convention is what this package implements).
// The client-side guard (frontend useGuardedMutation, Т2) closes the
// double-click window; this package covers what no client can: retries after
// timeouts, reloads, network replays.
package idempotency

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

var (
	// ErrKeyInProgress is the parallel-same-key race: the winner reserved the
	// key but has not completed its response yet. The loser answers 409
	// instead of executing a second creation.
	ErrKeyInProgress = errors.New("idempotency key request in progress")
	// ErrKeyBodyMismatch is «тот же ключ с другим телом» — a client bug
	// (keys are generated per logical attempt). The caller answers 409.
	ErrKeyBodyMismatch = errors.New("idempotency key reused with a different body")
)

// Replay is the stored result of the first attempt for a key.
type Replay struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

// Outcome of Reserve: either the caller won the key (execute the handler) or
// the first attempt's result replays.
type Outcome struct {
	Won    bool
	Replay Replay
}

// Service owns the idempotency_keys storage. Safe for concurrent use.
type Service struct {
	q *postgres.Queries
}

// NewService builds the service over a pgx pool/tx (postgres.DBTX).
func NewService(db postgres.DBTX) *Service {
	return &Service{q: postgres.New(db)}
}

// Reserve claims the key before the handler runs. The INSERT-first reservation
// closes the race: two parallel requests with the same key cannot both win,
// so the loser replays the winner's stored result or gets ErrKeyInProgress —
// never a second 201. A key whose response was never stored (the winner died
// mid-request) stays in progress until TTL cleanup: one key executes at most
// once, ever.
func (s *Service) Reserve(ctx context.Context, ownerID uuid.UUID, key, endpoint, requestHash string) (Outcome, error) {
	_, err := s.q.ReserveIdempotencyKey(ctx, postgres.ReserveIdempotencyKeyParams{
		OwnerID:     pgconv.UUIDToPgtype(ownerID),
		Key:         key,
		Endpoint:    endpoint,
		RequestHash: requestHash,
	})
	if err == nil {
		return Outcome{Won: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, fmt.Errorf("reserve idempotency key: %w", err)
	}
	existing, err := s.q.GetIdempotencyKey(ctx, postgres.GetIdempotencyKeyParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Key:     key,
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("load idempotency key: %w", err)
	}
	if !existing.StatusCode.Valid {
		return Outcome{}, ErrKeyInProgress
	}
	if existing.RequestHash != requestHash {
		return Outcome{}, ErrKeyBodyMismatch
	}
	return Outcome{Replay: Replay{
		StatusCode:  int(existing.StatusCode.Int32),
		ContentType: existing.ContentType.String,
		Body:        existing.Response,
	}}, nil
}

// Complete stores the winner's response. Best-effort by contract: a failure
// here must not fail the already-executed request (the caller logs it).
func (s *Service) Complete(ctx context.Context, ownerID uuid.UUID, key string, statusCode int, contentType string, body []byte) error {
	// HTTP-статус по RFC 9110 — три цифры; клампа закрывает теоретический
	// переполненный int (G115) заведомо невалидным значением, а не молчанием.
	status := statusCode
	if status < 0 || status > 599 {
		status = http.StatusInternalServerError
	}
	err := s.q.CompleteIdempotencyKey(ctx, postgres.CompleteIdempotencyKeyParams{
		OwnerID:     pgconv.UUIDToPgtype(ownerID),
		Key:         key,
		ContentType: pgtype.Text{String: contentType, Valid: contentType != ""},
		StatusCode:  pgtype.Int4{Int32: int32(status), Valid: true},
		Response:    body,
	})
	if err != nil {
		return fmt.Errorf("complete idempotency key: %w", err)
	}
	return nil
}

// Cleanup deletes keys older than the 24h TTL (the Stripe de-facto retention).
// Called opportunistically by the HTTP middleware — no dedicated queue.
func (s *Service) Cleanup(ctx context.Context) error {
	if _, err := s.q.CleanupExpiredIdempotencyKeys(ctx); err != nil {
		return fmt.Errorf("cleanup idempotency keys: %w", err)
	}
	return nil
}
