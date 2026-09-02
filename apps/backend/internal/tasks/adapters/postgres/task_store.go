package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.TaskStore = (*TaskStore)(nil)

// TaskStore is the postgres adapter of the tasks port (ADR 0051). Reads and
// writes are scoped by the data owner and the nested task→property path
// lives in the queries.
type TaskStore struct {
	db postgres.DBTX
}

// NewTaskStore creates a task store over the given connection or pool.
func NewTaskStore(db postgres.DBTX) *TaskStore {
	return &TaskStore{db: db}
}

func (s *TaskStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *TaskStore) WithTx(tx transaction.Tx) (application.TaskStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("tasks.TaskStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewTaskStore(dbtx), nil
}

// Get loads one task; pgx.ErrNoRows — an unknown id, another owner's task or
// another property's task — becomes the application ErrNotFound.
func (s *TaskStore) Get(
	ctx context.Context, id, scope, propertyID uuid.UUID,
) (domain.Task, error) {
	row, err := s.q().GetTask(ctx, postgres.GetTaskParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, application.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("get task %s: %w", id, err)
	}
	return mapTaskRow(taskFields{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		RuleID:        row.RuleID,
		DueDate:       row.DueDate,
		DueTime:       row.DueTime,
		Title:         row.Title,
		Comment:       row.Comment,
		CompletedDate: row.CompletedDate,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}), nil
}

// ListByProperty returns one page of the bucket's tasks with the total count
// of the same filter.
func (s *TaskStore) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID, q application.TasksListQuery,
) ([]domain.Task, int, error) {
	// The store re-checks the pagination bounds the service applied
	// (PrepareTasksQuery) before the int→int32 narrowing.
	if q.Limit < 1 || q.Limit > application.MaxTasksPageSize || q.Offset < 0 || q.Offset > math.MaxInt32 {
		return nil, 0, application.ErrInvalidInput
	}
	total, err := s.q().CountTasksByProperty(ctx, postgres.CountTasksByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Column3:    q.Completed,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count tasks of property %s: %w", propertyID, err)
	}
	params := postgres.ListActiveTasksByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Limit:      int32(q.Limit),
		Offset:     int32(q.Offset),
	}
	rows, err := s.list(ctx, q.Completed, params)
	if err != nil {
		return nil, 0, err
	}
	tasks := make([]domain.Task, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, mapTaskRow(taskFields{
			ID:            pgconv.UUIDFromPgtype(row.ID),
			OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
			PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
			RuleID:        row.RuleID,
			DueDate:       row.DueDate,
			DueTime:       row.DueTime,
			Title:         row.Title,
			Comment:       row.Comment,
			CompletedDate: row.CompletedDate,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		}))
	}
	return tasks, int(total), nil
}

// list dispatches the two bucket queries; the generated row shapes are
// identical, so one projection serves both.
func (s *TaskStore) list(
	ctx context.Context, completed bool, params postgres.ListActiveTasksByPropertyParams,
) ([]postgres.Task, error) {
	if completed {
		return s.q().ListCompletedTasksByProperty(ctx, postgres.ListCompletedTasksByPropertyParams(params))
	}
	return s.q().ListActiveTasksByProperty(ctx, params)
}

// Complete stamps the completion fact on the still-active task; rows
// affected = 0 surfaces as ErrAlreadyCompleted.
func (s *TaskStore) Complete(ctx context.Context, id, scope uuid.UUID, day time.Time) error {
	affected, err := s.q().CompleteTask(ctx, postgres.CompleteTaskParams{
		ID:            pgconv.UUIDToPgtype(id),
		CompletedDate: pgconv.DateToPgtype(day),
		OwnerID:       pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return fmt.Errorf("complete task %s: %w", id, err)
	}
	if affected == 0 {
		return application.ErrAlreadyCompleted
	}
	return nil
}

// Uncomplete clears the completion fact; rows affected = 0 surfaces as
// ErrNotCompleted.
func (s *TaskStore) Uncomplete(ctx context.Context, id, scope uuid.UUID) error {
	affected, err := s.q().UncompleteTask(ctx, postgres.UncompleteTaskParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return fmt.Errorf("uncomplete task %s: %w", id, err)
	}
	if affected == 0 {
		return application.ErrNotCompleted
	}
	return nil
}

// DeleteCompletedJournal removes the property's completed tasks of deleted
// rules and reports the count.
func (s *TaskStore) DeleteCompletedJournal(ctx context.Context, scope, propertyID uuid.UUID) (int64, error) {
	cleared, err := s.q().DeleteCompletedJournal(ctx, postgres.DeleteCompletedJournalParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return 0, fmt.Errorf("delete completed journal of property %s: %w", propertyID, err)
	}
	return cleared, nil
}
