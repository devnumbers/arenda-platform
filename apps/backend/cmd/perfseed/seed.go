package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type seedConfig struct {
	ownerCount int
}

type ownerState struct {
	id                   uuid.UUID
	phone                string
	propertyID           uuid.UUID
	leaseID              uuid.UUID
	operationID          uuid.UUID
	recurringOperationID uuid.UUID
	reminderID           uuid.UUID
	tenantContactID      uuid.UUID
	sessionToken         string
}

type seedState struct {
	owners []ownerState
}

type serializableOwnerState struct {
	Index                int    `json:"index"`
	ID                   string `json:"id"`
	Phone                string `json:"phone"`
	SessionToken         string `json:"sessionToken"`
	PropertyID           string `json:"propertyID"`
	LeaseID              string `json:"leaseID"`
	OperationID          string `json:"operationID"`
	RecurringOperationID string `json:"recurringOperationID"`
	ReminderID           string `json:"reminderID"`
	TenantContactID      string `json:"tenantContactID"`
}

type fixturesFile struct {
	Owners []serializableOwnerState `json:"owners"`
}

func writeFixtures(state *seedState) error {
	owners := make([]serializableOwnerState, len(state.owners))
	for i, o := range state.owners {
		owners[i] = serializableOwnerState{
			Index:                i,
			ID:                   o.id.String(),
			Phone:                o.phone,
			SessionToken:         o.sessionToken,
			PropertyID:           o.propertyID.String(),
			LeaseID:              o.leaseID.String(),
			OperationID:          o.operationID.String(),
			RecurringOperationID: o.recurringOperationID.String(),
			ReminderID:           o.reminderID.String(),
			TenantContactID:      o.tenantContactID.String(),
		}
	}
	data, err := json.MarshalIndent(fixturesFile{Owners: owners}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fixtures: %w", err)
	}
	if err := os.WriteFile("perf/fixtures.json", data, 0600); err != nil {
		return fmt.Errorf("write fixtures: %w", err)
	}
	return nil
}

func resetDB(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		TRUNCATE TABLE
			sent_sms_reminders,
			subscription_payments,
			payment_methods,
			user_subscriptions,
			tariffs,
			reminders,
			recurring_operations,
			operations,
			leases,
			tenant_contacts,
			properties,
			sessions,
			login_attempts,
			sms_codes,
			users
		CASCADE
	`)
	return err
}

func seedBase(ctx context.Context, db *pgxpool.Pool, cfg seedConfig) (*seedState, error) {
	state := &seedState{owners: make([]ownerState, cfg.ownerCount)}

	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO tariffs (id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks)
		VALUES ($1, 'basic', 100, 0, 0)
		ON CONFLICT (name) DO UPDATE SET
			active_property_limit = EXCLUDED.active_property_limit,
			monthly_price_kopecks = EXCLUDED.monthly_price_kopecks,
			yearly_price_kopecks = EXCLUDED.yearly_price_kopecks
	`, uuid.New()); err != nil {
		return nil, fmt.Errorf("seed tariff: %w", err)
	}

	for i := 0; i < cfg.ownerCount; i++ {
		owner, err := seedOwner(ctx, tx, i)
		if err != nil {
			return nil, fmt.Errorf("seed owner %d: %w", i, err)
		}
		state.owners[i] = *owner
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return state, nil
}

func seedOwner(ctx context.Context, tx pgx.Tx, index int) (*ownerState, error) {
	ownerID := uuid.New()
	phone := fmt.Sprintf("+7999%07d", index)
	token := deterministicToken(index)
	tokenHash := hashToken(token)
	now := time.Now().UTC()

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, phone, role, created_at, updated_at)
		VALUES ($1, $2, 'owner', $3, $3)
	`, ownerID, phone, now); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_subscriptions (id, user_id, tariff_id, source, status, valid_until, created_at, updated_at)
		VALUES ($1, $2, (SELECT id FROM tariffs WHERE name = 'basic' LIMIT 1), 'service', 'active', $3, $4, $4)
	`, uuid.New(), ownerID, now.Add(365*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), ownerID, tokenHash, now.Add(30*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	propertyID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO properties (id, owner_id, name, type, address, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'apartment', $4, 'active', $5, $5)
	`, propertyID, ownerID, fmt.Sprintf("Property %d", index), fmt.Sprintf("Address %d", index), now); err != nil {
		return nil, fmt.Errorf("insert property: %w", err)
	}

	contactID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_contacts (id, owner_id, name, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, contactID, ownerID, fmt.Sprintf("Contact %d", index), fmt.Sprintf("+7955%07d", index), now); err != nil {
		return nil, fmt.Errorf("insert tenant contact: %w", err)
	}

	leaseID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO leases (
			id, owner_id, property_id, tenant_contact_id, status,
			start_date, end_date, rent_amount_kopecks, deposit_amount_kopecks, payment_day,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'active', $5, $6, 5000000, 0, 1, $7, $7)
	`, leaseID, ownerID, propertyID, contactID,
		now.Add(-30*24*time.Hour).Truncate(24*time.Hour),
		now.Add(335*24*time.Hour).Truncate(24*time.Hour),
		now); err != nil {
		return nil, fmt.Errorf("insert lease: %w", err)
	}

	opID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO operations (
			id, owner_id, property_id, lease_id, type, category,
			amount_kopecks, operation_date, is_exception, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'expense', 'repair', 100000, $5, false, $6, $6)
	`, opID, ownerID, propertyID, leaseID,
		now.Truncate(24*time.Hour),
		now); err != nil {
		return nil, fmt.Errorf("insert operation: %w", err)
	}

	recID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO recurring_operations (
			id, owner_id, property_id, lease_id, type, category,
			amount_kopecks, start_date, payment_day, periodicity, status,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, NULL, 'expense', 'utilities', 200000, $4, 1, 'monthly', 'active', $5, $5)
	`, recID, ownerID, propertyID,
		now.Add(-60*24*time.Hour).Truncate(24*time.Hour),
		now); err != nil {
		return nil, fmt.Errorf("insert recurring operation: %w", err)
	}

	remID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO reminders (
			id, owner_id, target_type, operation_id, event_type, status,
			scheduled_at, message_title, message_body, created_at, updated_at
		)
		VALUES ($1, $2, 'operation', $3, 'operation_due', 'pending', $4, $5, $6, $7, $7)
	`, remID, ownerID, opID,
		now.Add(7*24*time.Hour),
		fmt.Sprintf("Operation due reminder %d", index),
		fmt.Sprintf("Reminder body %d", index),
		now); err != nil {
		return nil, fmt.Errorf("insert reminder: %w", err)
	}

	return &ownerState{
		id:                   ownerID,
		phone:                phone,
		propertyID:           propertyID,
		leaseID:              leaseID,
		operationID:          opID,
		recurringOperationID: recID,
		reminderID:           remID,
		tenantContactID:      contactID,
		sessionToken:         token,
	}, nil
}

func seedEndpoint(ctx context.Context, db *pgxpool.Pool, state *seedState, endpoint string) error {
	switch endpoint {
	case "delete_reminder":
		return seedManyReminders(ctx, db, state)
	case "auth_verify_code":
		return seedAuthCodes(ctx, db, state)
	default:
		return fmt.Errorf("unknown perfseed endpoint: %s", endpoint)
	}
}

func seedManyReminders(ctx context.Context, db *pgxpool.Pool, state *seedState) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	batch := &pgx.Batch{}

	const remindersPerOwner = 10
	for _, owner := range state.owners {
		for i := 0; i < remindersPerOwner; i++ {
			scheduledAt := now.Add(time.Duration(i+1) * 24 * time.Hour)
			batch.Queue(`
				INSERT INTO reminders (
					id, owner_id, target_type, operation_id, event_type, status,
					scheduled_at, message_title, message_body, created_at, updated_at
				)
				VALUES ($1, $2, 'operation', $3, 'operation_due', 'pending', $4, $5, $6, $7, $7)
			`, uuid.New(), owner.id, owner.operationID, scheduledAt,
				fmt.Sprintf("Operation due reminder extra %d", i),
				fmt.Sprintf("Reminder extra body %d", i),
				now)
		}
	}

	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert reminders: %w", err)
	}
	return tx.Commit(ctx)
}

func seedAuthCodes(ctx context.Context, db *pgxpool.Pool, state *seedState) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	code := "000000"
	codeHash := hashToken(code)
	expiresAt := now.Add(2 * time.Hour)
	batch := &pgx.Batch{}

	for _, owner := range state.owners {
		batch.Queue(`
			INSERT INTO sms_codes (id, user_id, phone, code_hash, expires_at, used, created_at)
			VALUES ($1, $2, $3, $4, $5, false, $6)
		`, uuid.New(), owner.id, owner.phone, codeHash, expiresAt, now)
	}

	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert sms codes: %w", err)
	}
	return tx.Commit(ctx)
}

func deterministicToken(index int) string {
	return fmt.Sprintf("perf-session-token-%d", index)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
