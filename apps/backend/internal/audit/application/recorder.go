// Package application provides the audit recording port used by other modules
// and its default service implementation.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// EntryWriter persists audit entries. Implemented by persistence adapters.
type EntryWriter interface {
	Insert(ctx context.Context, entry domain.Entry) error
	WithTx(tx transaction.Tx) EntryWriter
}

// Recorder is the port other modules use to write audit entries.
type Recorder interface {
	Record(ctx context.Context, entry domain.Entry) error
	WithTx(tx transaction.Tx) Recorder
}

// Service is the default Recorder implementation.
type Service struct {
	writer EntryWriter
	clock  clock.Clock
}

var _ Recorder = (*Service)(nil)

// NewService creates a Service.
func NewService(writer EntryWriter, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Service{writer: writer, clock: clk}
}

// WithTx returns a Recorder bound to the provided transaction.
func (s *Service) WithTx(tx transaction.Tx) Recorder {
	return &Service{writer: s.writer.WithTx(tx), clock: s.clock}
}

// Record assigns the entry ID and timestamp, fills request diagnostics from the
// context, and persists the entry. Insert errors are returned to the caller so
// that transaction-bound callers roll back (fail-safe semantics).
func (s *Service) Record(ctx context.Context, entry domain.Entry) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate audit id: %w", err)
	}
	entry.ID = id
	entry.CreatedAt = s.clock.Now()
	if entry.Context == nil {
		entry.Context = map[string]any{}
	}
	if entry.RequestID == "" {
		entry.RequestID = requestctx.RequestIDFromContext(ctx)
	}
	if entry.IP == "" {
		entry.IP = requestctx.ClientIPFromContext(ctx)
	}
	if err := s.writer.Insert(ctx, entry); err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}

// Noop is a Recorder that discards entries.
type Noop struct{}

var _ Recorder = Noop{}

// Record discards the entry.
func (Noop) Record(_ context.Context, _ domain.Entry) error { return nil }

// WithTx returns the same Noop recorder.
func (n Noop) WithTx(_ transaction.Tx) Recorder { return n }
