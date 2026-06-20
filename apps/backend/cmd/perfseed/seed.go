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
	ownerCount                     int
	propertiesPerOwner             int
	operationsPerProperty          int
	recurringOperationsPerProperty int
	remindersPerTarget             int
	tenantContactsPerOwner         int
	paymentMethodsPerOwner         int
	subscriptionPaymentsPerOwner   int
}

type ownerState struct {
	id                   uuid.UUID
	subscriptionID       uuid.UUID
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
	Tokens               []string                 `json:"tokens"`
	OwnerIDs             []string                 `json:"owner_ids"`
	PropertyID           string                   `json:"property_id"`
	LeaseID              string                   `json:"lease_id"`
	OperationID          string                   `json:"operation_id"`
	RecurringOperationID string                   `json:"recurring_operation_id"`
	ReminderID           string                   `json:"reminder_id"`
	ContactID            string                   `json:"contact_id"`
	Owners               []serializableOwnerState `json:"owners"`
}

func writeFixtures(state *seedState) error {
	owners := make([]serializableOwnerState, len(state.owners))
	tokens := make([]string, len(state.owners))
	ownerIDs := make([]string, len(state.owners))
	for i, o := range state.owners {
		tokens[i] = o.sessionToken
		ownerIDs[i] = o.id.String()
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

	var primary ownerState
	if len(state.owners) > 0 {
		primary = state.owners[0]
	}

	data, err := json.MarshalIndent(fixturesFile{
		Tokens:               tokens,
		OwnerIDs:             ownerIDs,
		PropertyID:           primary.propertyID.String(),
		LeaseID:              primary.leaseID.String(),
		OperationID:          primary.operationID.String(),
		RecurringOperationID: primary.recurringOperationID.String(),
		ReminderID:           primary.reminderID.String(),
		ContactID:            primary.tenantContactID.String(),
		Owners:               owners,
	}, "", "  ")
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

	if _, err := tx.Exec(ctx, `
		INSERT INTO tariffs (id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks)
		VALUES ($1, 'pro', 100, 100000, 1000000)
		ON CONFLICT (name) DO UPDATE SET
			active_property_limit = EXCLUDED.active_property_limit,
			monthly_price_kopecks = EXCLUDED.monthly_price_kopecks,
			yearly_price_kopecks = EXCLUDED.yearly_price_kopecks
	`, uuid.New()); err != nil {
		return nil, fmt.Errorf("seed pro tariff: %w", err)
	}

	for i := 0; i < cfg.ownerCount; i++ {
		owner, err := seedOwner(ctx, tx, cfg, i)
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

func seedOwner(ctx context.Context, tx pgx.Tx, cfg seedConfig, index int) (*ownerState, error) {
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

	subscriptionID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_subscriptions (id, user_id, tariff_id, source, status, valid_until, created_at, updated_at)
		VALUES ($1, $2, (SELECT id FROM tariffs WHERE name = 'basic' LIMIT 1), 'paid', 'active', $3, $4, $4)
	`, subscriptionID, ownerID, now.Add(365*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), ownerID, tokenHash, now.Add(30*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	state := &ownerState{
		id:             ownerID,
		subscriptionID: subscriptionID,
		phone:          phone,
		sessionToken:   token,
	}

	contactIDs, err := seedTenantContacts(ctx, tx, ownerID, index, cfg.tenantContactsPerOwner, now)
	if err != nil {
		return nil, err
	}
	state.tenantContactID = contactIDs[0]

	paymentMethodIDs, err := seedPaymentMethods(ctx, tx, ownerID, index, cfg.paymentMethodsPerOwner, now)
	if err != nil {
		return nil, err
	}
	if err := seedSubscriptionPayments(ctx, tx, ownerID, subscriptionID, paymentMethodIDs, index, cfg.subscriptionPaymentsPerOwner, now); err != nil {
		return nil, err
	}

	for propertyIndex := 0; propertyIndex < cfg.propertiesPerOwner; propertyIndex++ {
		propertyID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO properties (id, owner_id, name, type, address, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'apartment', $4, 'active', $5, $5)
		`, propertyID, ownerID, fmt.Sprintf("Property %d-%d", index, propertyIndex), fmt.Sprintf("Address %d-%d", index, propertyIndex), now); err != nil {
			return nil, fmt.Errorf("insert property: %w", err)
		}

		contactID := contactIDs[propertyIndex%len(contactIDs)]
		leaseID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO leases (
				id, owner_id, property_id, tenant_contact_id, status,
				start_date, end_date, rent_amount_kopecks, deposit_amount_kopecks, payment_day,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, 0, $8, $9, $9)
		`, leaseID, ownerID, propertyID, contactID,
			now.Add(-30*24*time.Hour).Truncate(24*time.Hour),
			now.Add(335*24*time.Hour).Truncate(24*time.Hour),
			int64(4_500_000+propertyIndex*250_000),
			1+(propertyIndex%28),
			now); err != nil {
			return nil, fmt.Errorf("insert lease: %w", err)
		}

		operationIDs, err := seedOperations(ctx, tx, ownerID, propertyID, leaseID, propertyIndex, cfg.operationsPerProperty, now)
		if err != nil {
			return nil, err
		}

		recurringIDs, err := seedRecurringOperations(ctx, tx, ownerID, propertyID, leaseID, propertyIndex, cfg.recurringOperationsPerProperty, now)
		if err != nil {
			return nil, err
		}

		if propertyIndex == 0 {
			state.propertyID = propertyID
			state.leaseID = leaseID
			state.operationID = operationIDs[0]
			state.recurringOperationID = recurringIDs[0]
		}

		firstReminderID, err := seedReminders(ctx, tx, ownerID, propertyID, leaseID, operationIDs[0], recurringIDs[0], index, propertyIndex, cfg.remindersPerTarget, now)
		if err != nil {
			return nil, err
		}
		if propertyIndex == 0 {
			state.reminderID = firstReminderID
		}
	}

	return state, nil
}

func seedTenantContacts(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, ownerIndex int, count int, now time.Time) ([]uuid.UUID, error) {
	count = maxInt(count, 1)
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		contactID := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO tenant_contacts (id, owner_id, name, phone, email, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
		`,
			contactID,
			ownerID,
			fmt.Sprintf("Contact %d-%d", ownerIndex, i),
			fmt.Sprintf("+7955%04d%03d", ownerIndex%10_000, i),
			fmt.Sprintf("tenant-%d-%d@example.test", ownerIndex, i),
			now,
		); err != nil {
			return nil, fmt.Errorf("insert tenant contact: %w", err)
		}
		ids = append(ids, contactID)
	}
	return ids, nil
}

func seedOperations(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, propertyID uuid.UUID, leaseID uuid.UUID, propertyIndex int, count int, now time.Time) ([]uuid.UUID, error) {
	count = maxInt(count, 1)
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		opID := uuid.New()
		opType := "expense"
		category := "repair"
		if i%6 == 0 {
			opType = "income"
			category = "rent"
		} else if i%3 == 0 {
			category = "utilities"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO operations (
				id, owner_id, property_id, lease_id, type, category,
				amount_kopecks, operation_date, is_exception, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, $9, $9)
		`,
			opID,
			ownerID,
			propertyID,
			leaseID,
			opType,
			category,
			int64(75_000+propertyIndex*10_000+i*2_500),
			now.AddDate(0, 0, -i).Truncate(24*time.Hour),
			now,
		); err != nil {
			return nil, fmt.Errorf("insert operation: %w", err)
		}
		ids = append(ids, opID)
	}
	return ids, nil
}

func seedRecurringOperations(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, propertyID uuid.UUID, leaseID uuid.UUID, propertyIndex int, count int, now time.Time) ([]uuid.UUID, error) {
	count = maxInt(count, 1)
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		recID := uuid.New()
		category := "utilities"
		if i%2 == 1 {
			category = "rent"
		}
		opType := "expense"
		if category == "rent" {
			opType = "income"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO recurring_operations (
				id, owner_id, property_id, lease_id, type, category,
				amount_kopecks, start_date, payment_day, periodicity, status,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'monthly', 'active', $10, $10)
		`,
			recID,
			ownerID,
			propertyID,
			leaseID,
			opType,
			category,
			int64(120_000+propertyIndex*20_000+i*5_000),
			now.AddDate(0, -3, -i).Truncate(24*time.Hour),
			1+(i%28),
			now,
		); err != nil {
			return nil, fmt.Errorf("insert recurring operation: %w", err)
		}
		ids = append(ids, recID)
	}
	return ids, nil
}

func seedReminders(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, propertyID uuid.UUID, leaseID uuid.UUID, operationID uuid.UUID, recurringID uuid.UUID, ownerIndex int, propertyIndex int, count int, now time.Time) (uuid.UUID, error) {
	count = maxInt(count, 1)
	var first uuid.UUID
	for i := 0; i < count; i++ {
		reminderID, err := insertReminder(ctx, tx, ownerID, propertyID, "operation", operationID, "operation_due", now.Add(time.Duration(24+i)*time.Hour), fmt.Sprintf("Operation due %d-%d-%d", ownerIndex, propertyIndex, i), now)
		if err != nil {
			return uuid.Nil, err
		}
		if i == 0 {
			first = reminderID
		}
		if _, err := insertReminder(ctx, tx, ownerID, propertyID, "recurring_operation", recurringID, "operation_due", now.Add(time.Duration(48+i)*time.Hour), fmt.Sprintf("Recurring operation due %d-%d-%d", ownerIndex, propertyIndex, i), now); err != nil {
			return uuid.Nil, err
		}
		if _, err := insertReminder(ctx, tx, ownerID, propertyID, "lease", leaseID, "lease_expiring", now.Add(time.Duration(72+i)*time.Hour), fmt.Sprintf("Lease expiring %d-%d-%d", ownerIndex, propertyIndex, i), now); err != nil {
			return uuid.Nil, err
		}
	}
	return first, nil
}

func insertReminder(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, propertyID uuid.UUID, targetType string, targetID uuid.UUID, eventType string, scheduledAt time.Time, title string, now time.Time) (uuid.UUID, error) {
	var operationID any
	var recurringID any
	var leaseID any
	switch targetType {
	case "operation":
		operationID = targetID
	case "recurring_operation":
		recurringID = targetID
	case "lease":
		leaseID = targetID
	default:
		return uuid.Nil, fmt.Errorf("unsupported reminder target type %q", targetType)
	}

	remID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO reminders (
			id, owner_id, target_type, operation_id, recurring_operation_id, lease_id, property_id,
			event_type, status, scheduled_at, message_title, message_body, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9, $10, $11, $12, $12)
	`,
		remID,
		ownerID,
		targetType,
		operationID,
		recurringID,
		leaseID,
		propertyID,
		eventType,
		scheduledAt,
		title,
		"Perf reminder body",
		now,
	); err != nil {
		return uuid.Nil, fmt.Errorf("insert reminder: %w", err)
	}
	return remID, nil
}

func seedPaymentMethods(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, ownerIndex int, count int, now time.Time) ([]uuid.UUID, error) {
	count = maxInt(count, 1)
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		methodID := uuid.New()
		providerToken := fmt.Sprintf("perf-payment-token-%d-%d", ownerIndex, i)
		if _, err := tx.Exec(ctx, `
			INSERT INTO payment_methods (
				id, user_id, provider, provider_token, token_hash, display_mask, is_active, created_at, updated_at
			)
			VALUES ($1, $2, 'fake', $3, $4, $5, $6, $7, $7)
		`,
			methodID,
			ownerID,
			providerToken,
			hashToken(providerToken),
			fmt.Sprintf("**** %04d", (ownerIndex+i)%10_000),
			i == 0,
			now,
		); err != nil {
			return nil, fmt.Errorf("insert payment method: %w", err)
		}
		ids = append(ids, methodID)
	}
	return ids, nil
}

func seedSubscriptionPayments(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, subscriptionID uuid.UUID, paymentMethodIDs []uuid.UUID, ownerIndex int, count int, now time.Time) error {
	count = maxInt(count, 1)
	for i := 0; i < count; i++ {
		paymentID := uuid.New()
		status := "succeeded"
		if i%5 == 4 {
			status = "failed"
		}
		period := "month"
		if i%4 == 3 {
			period = "year"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO subscription_payments (
				id, user_id, subscription_id, tariff_id, payment_method_id, period,
				amount_kopecks, provider, provider_payment_id, status, error_code, created_at, updated_at
			)
			VALUES ($1, $2, $3, (SELECT id FROM tariffs WHERE name = 'pro' LIMIT 1), $4, $5, $6, 'fake', $7, $8, $9, $10, $10)
		`,
			paymentID,
			ownerID,
			subscriptionID,
			paymentMethodIDs[i%len(paymentMethodIDs)],
			period,
			int64(100_000+i*10_000),
			fmt.Sprintf("perf-subscription-payment-%d-%d", ownerIndex, i),
			status,
			errorCodeForStatus(status),
			now.AddDate(0, -i, 0),
		); err != nil {
			return fmt.Errorf("insert subscription payment: %w", err)
		}
	}
	return nil
}

func errorCodeForStatus(status string) any {
	if status != "failed" {
		return nil
	}
	return "fake_declined"
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func deterministicToken(index int) string {
	return fmt.Sprintf("perf-session-token-%d", index)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
