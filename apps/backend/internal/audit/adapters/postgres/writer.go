// Package postgres implements audit entry persistence over PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/jackc/pgx/v5/pgtype"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Writer persists audit entries via sqlc-generated queries.
type Writer struct {
	db postgres.DBTX
}

var _ auditapp.EntryWriter = (*Writer)(nil)

// NewWriter creates a new audit entry writer.
func NewWriter(db postgres.DBTX) *Writer {
	return &Writer{db: db}
}

func (w *Writer) q() *postgres.Queries {
	return postgres.New(w.db)
}

// WithTx returns a writer bound to the provided transaction. A transaction
// that is not a postgres.DBTX yields a writer whose Insert always fails, so
// the error propagates through Record instead of panicking.
func (w *Writer) WithTx(tx transaction.Tx) auditapp.EntryWriter {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return failingWriter{err: fmt.Errorf("audit.Writer.WithTx: %T is not a postgres.DBTX", tx)}
	}
	return NewWriter(dbtx)
}

// failingWriter is an EntryWriter whose Insert always returns err. WithTx
// returns it when the given transaction is not a postgres.DBTX.
type failingWriter struct {
	err error
}

func (f failingWriter) Insert(_ context.Context, _ domain.Entry) error { return f.err }

func (f failingWriter) WithTx(_ transaction.Tx) auditapp.EntryWriter { return f }

// Insert writes one audit entry.
func (w *Writer) Insert(ctx context.Context, entry domain.Entry) error {
	contextJSON, err := json.Marshal(entry.Context)
	if err != nil {
		return fmt.Errorf("marshal audit context: %w", err)
	}
	_, err = w.q().InsertAuditLog(ctx, postgres.InsertAuditLogParams{
		ID:         pgconv.UUIDToPgtype(entry.ID),
		CreatedAt:  pgtype.Timestamptz{Time: entry.CreatedAt, Valid: true},
		ActorID:    pgconv.UUIDToPgtypePtr(entry.ActorID),
		ActorRole:  string(entry.ActorRole),
		Action:     string(entry.Action),
		EntityType: pgtype.Text{String: string(entry.EntityType), Valid: entry.EntityType != ""},
		EntityID:   pgconv.UUIDToPgtypePtr(entry.EntityID),
		Context:    contextJSON,
		RequestID:  pgtype.Text{String: entry.RequestID, Valid: entry.RequestID != ""},
		Ip:         parseIP(entry.IP),
	})
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// parseIP converts the stored IP string to netip.Addr. Empty or unparsable
// values yield nil: a malformed IP must not fail the audit insert.
func parseIP(s string) *netip.Addr {
	if s == "" {
		return nil
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &addr
}
