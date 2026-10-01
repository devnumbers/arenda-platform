package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapters to the consumer-declared ports.
var (
	_ application.RuleStore     = (*RuleStore)(nil)
	_ application.PropertyStore = (*PropertyStore)(nil)
)

// RuleStore is the postgres adapter of the task rules port (ADR 0051).
// Reads and writes are scoped by the data owner and the nested rule→property
// path lives in the queries.
type RuleStore struct {
	db postgres.DBTX
}

// NewRuleStore creates a rule store over the given connection or pool.
func NewRuleStore(db postgres.DBTX) *RuleStore {
	return &RuleStore{db: db}
}

func (s *RuleStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *RuleStore) WithTx(tx transaction.Tx) (application.RuleStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("tasks.RuleStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewRuleStore(dbtx), nil
}

// Get loads one rule; pgx.ErrNoRows — an unknown id, another owner's rule or
// another property's rule — becomes the application ErrNotFound.
func (s *RuleStore) Get(
	ctx context.Context, id, scope, propertyID uuid.UUID,
) (domain.TaskRule, error) {
	row, err := s.q().GetTaskRule(ctx, postgres.GetTaskRuleParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TaskRule{}, application.ErrNotFound
		}
		return domain.TaskRule{}, fmt.Errorf("get task rule %s: %w", id, err)
	}
	return mapRuleRow(taskRuleFields{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    row.PropertyID,
		Title:         row.Title,
		Comment:       row.Comment,
		DueDate:       row.DueDate,
		DueTime:       row.DueTime,
		Repeat:        row.Repeat,
		HistoryBefore: row.HistoryBefore,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}), nil
}

// GetWithoutProperty loads one rule of the property-less cut (ADR 0052);
// pgx.ErrNoRows — an unknown id, another owner's rule or a bound rule (the
// slices never mix) — becomes the application ErrNotFound.
func (s *RuleStore) GetWithoutProperty(
	ctx context.Context, id, scope uuid.UUID,
) (domain.TaskRule, error) {
	row, err := s.q().GetTaskRuleWithoutProperty(ctx, postgres.GetTaskRuleWithoutPropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TaskRule{}, application.ErrNotFound
		}
		return domain.TaskRule{}, fmt.Errorf("get task rule %s without property: %w", id, err)
	}
	return mapRuleRow(taskRuleFields{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    row.PropertyID,
		Title:         row.Title,
		Comment:       row.Comment,
		DueDate:       row.DueDate,
		DueTime:       row.DueTime,
		Repeat:        row.Repeat,
		HistoryBefore: row.HistoryBefore,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}), nil
}

// Create inserts a new rule (id, owner_id are app-side, ADR 0019/0028); a
// nil property creates the rule in the owner's own book (ADR 0052).
func (s *RuleStore) Create(ctx context.Context, rule domain.TaskRule) error {
	if err := s.q().CreateTaskRule(ctx, postgres.CreateTaskRuleParams{
		ID:         pgconv.UUIDToPgtype(rule.ID),
		OwnerID:    pgconv.UUIDToPgtype(rule.OwnerID),
		PropertyID: pgconv.UUIDToPgtypePtr(rule.PropertyID),
		Title:      rule.Title,
		Comment:    pgconv.StringPtrToPgtype(rule.Comment),
		DueDate:    pgconv.DatePtrToPgtype(rule.DueDate),
		DueTime:    timeOfDayToPgtype(rule.DueTime),
		Repeat:     string(rule.Repeat),
	}); err != nil {
		return fmt.Errorf("create task rule %s: %w", rule.ID, err)
	}
	return nil
}

// Update writes the editable fields of the rule (the anchor included — the
// edit invalidation and the in-transaction tick resolve the task side) and
// the history horizon the rule carries: a dated edit keeps it, an undated
// one passes NULL (the application resets the dormancy marker before this).
func (s *RuleStore) Update(ctx context.Context, rule domain.TaskRule) error {
	if err := s.q().UpdateTaskRule(ctx, postgres.UpdateTaskRuleParams{
		ID:            pgconv.UUIDToPgtype(rule.ID),
		Title:         rule.Title,
		Comment:       pgconv.StringPtrToPgtype(rule.Comment),
		DueDate:       pgconv.DatePtrToPgtype(rule.DueDate),
		DueTime:       timeOfDayToPgtype(rule.DueTime),
		Repeat:        string(rule.Repeat),
		HistoryBefore: pgconv.DatePtrToPgtype(rule.HistoryBefore),
		OwnerID:       pgconv.UUIDToPgtype(rule.OwnerID),
	}); err != nil {
		return fmt.Errorf("update task rule %s: %w", rule.ID, err)
	}
	return nil
}

// Delete removes the rule row; the completed journal keeps its snapshots
// with rule_id set to NULL by the FK.
func (s *RuleStore) Delete(ctx context.Context, id, scope uuid.UUID) error {
	if err := s.q().DeleteTaskRule(ctx, postgres.DeleteTaskRuleParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	}); err != nil {
		return fmt.Errorf("delete task rule %s: %w", id, err)
	}
	return nil
}

// DeleteNotDueUncompleted removes the rule's uncompleted tasks that have not
// fallen due yet (the undated one and the strictly future ones) — the edit
// invalidation; the in-transaction tick stands the single future again.
func (s *RuleStore) DeleteNotDueUncompleted(ctx context.Context, ruleID uuid.UUID, today time.Time) error {
	if err := s.q().DeleteRuleNotDueUncompleted(ctx, postgres.DeleteRuleNotDueUncompletedParams{
		RuleID:  pgconv.UUIDToPgtype(ruleID),
		DueDate: pgconv.DateToPgtype(today),
	}); err != nil {
		return fmt.Errorf("delete not-due uncompleted tasks of rule %s: %w", ruleID, err)
	}
	return nil
}

// DeleteUncompleted removes every uncompleted task of the rule — the hard
// rule deletion semantics (resolution #496).
func (s *RuleStore) DeleteUncompleted(ctx context.Context, ruleID uuid.UUID) error {
	if err := s.q().DeleteRuleUncompleted(ctx, pgconv.UUIDToPgtype(ruleID)); err != nil {
		return fmt.Errorf("delete uncompleted tasks of rule %s: %w", ruleID, err)
	}
	return nil
}

// PropertyStore is the postgres adapter of the property serialization port.
type PropertyStore struct {
	db postgres.DBTX
}

// NewPropertyStore creates a property store over the given connection or
// pool.
func NewPropertyStore(db postgres.DBTX) *PropertyStore {
	return &PropertyStore{db: db}
}

func (s *PropertyStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *PropertyStore) WithTx(tx transaction.Tx) (application.PropertyStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("tasks.PropertyStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewPropertyStore(dbtx), nil
}

// Get loads the property reference without locking; the display name rides
// along as the global listing's row projection (ticket #521).
func (s *PropertyStore) Get(ctx context.Context, propertyID uuid.UUID) (application.PropertyRef, error) {
	row, err := s.q().GetPropertyForTask(ctx, pgconv.UUIDToPgtype(propertyID))
	return propertyRefFromRow(row.OwnerID, row.Status, row.Name, err, propertyID)
}

// GetForUpdate loads the property reference with the row locked inside the
// caller's transaction — the context's serialization point.
func (s *PropertyStore) GetForUpdate(ctx context.Context, propertyID uuid.UUID) (application.PropertyRef, error) {
	row, err := s.q().GetPropertyForTaskMutation(ctx, pgconv.UUIDToPgtype(propertyID))
	return propertyRefFromRow(row.OwnerID, row.Status, "", err, propertyID)
}

// propertyRefFromRow maps one property read onto the application reference;
// the archived flag is the status string's read view, pgx.ErrNoRows is the
// privacy ErrNotFound. The name is empty on the reads that don't project it.
func propertyRefFromRow(
	ownerID pgtype.UUID, status, name string, err error, propertyID uuid.UUID,
) (application.PropertyRef, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PropertyRef{}, application.ErrNotFound
		}
		return application.PropertyRef{}, fmt.Errorf("load property %s: %w", propertyID, err)
	}
	return application.PropertyRef{
		OwnerID:  pgconv.UUIDFromPgtype(ownerID),
		Archived: status == "archived",
		Name:     name,
	}, nil
}
