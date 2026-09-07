package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
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
		PropertyID:    row.PropertyID,
		RuleID:        row.RuleID,
		DueDate:       row.DueDate,
		DueTime:       row.DueTime,
		Title:         row.Title,
		Comment:       row.Comment,
		CompletedDate: row.CompletedDate,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		RuleRepeat:    row.RuleRepeat,
	}), nil
}

// GetWithoutProperty loads one task of the property-less cut (ADR 0052);
// pgx.ErrNoRows — an unknown id, another owner's task or a bound task (the
// slices never mix) — becomes the application ErrNotFound.
func (s *TaskStore) GetWithoutProperty(
	ctx context.Context, id, scope uuid.UUID,
) (domain.Task, error) {
	row, err := s.q().GetTaskWithoutProperty(ctx, postgres.GetTaskWithoutPropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, application.ErrNotFound
		}
		return domain.Task{}, fmt.Errorf("get task %s without property: %w", id, err)
	}
	return mapTaskRow(taskFields{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    row.PropertyID,
		RuleID:        row.RuleID,
		DueDate:       row.DueDate,
		DueTime:       row.DueTime,
		Title:         row.Title,
		Comment:       row.Comment,
		CompletedDate: row.CompletedDate,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		RuleRepeat:    row.RuleRepeat,
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
	tasks, err := s.list(ctx, q.Completed, params)
	if err != nil {
		return nil, 0, err
	}
	return tasks, int(total), nil
}

// list dispatches the two bucket queries; the generated row shapes are
// identical (both carry the joined rule repeat), so one projection serves
// both — each branch maps onto the shared taskFields.
func (s *TaskStore) list(
	ctx context.Context, completed bool, params postgres.ListActiveTasksByPropertyParams,
) ([]domain.Task, error) {
	if completed {
		rows, err := s.q().ListCompletedTasksByProperty(ctx, postgres.ListCompletedTasksByPropertyParams(params))
		if err != nil {
			return nil, fmt.Errorf("list completed tasks: %w", err)
		}
		tasks := make([]domain.Task, 0, len(rows))
		for _, row := range rows {
			tasks = append(tasks, mapTaskRow(taskFields{
				ID:            pgconv.UUIDFromPgtype(row.ID),
				OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
				PropertyID:    row.PropertyID,
				RuleID:        row.RuleID,
				DueDate:       row.DueDate,
				DueTime:       row.DueTime,
				Title:         row.Title,
				Comment:       row.Comment,
				CompletedDate: row.CompletedDate,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
				RuleRepeat:    row.RuleRepeat,
			}))
		}
		return tasks, nil
	}
	rows, err := s.q().ListActiveTasksByProperty(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	tasks := make([]domain.Task, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, mapTaskRow(taskFields{
			ID:            pgconv.UUIDFromPgtype(row.ID),
			OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
			PropertyID:    row.PropertyID,
			RuleID:        row.RuleID,
			DueDate:       row.DueDate,
			DueTime:       row.DueTime,
			Title:         row.Title,
			Comment:       row.Comment,
			CompletedDate: row.CompletedDate,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			RuleRepeat:    row.RuleRepeat,
		}))
	}
	return tasks, nil
}

// ListGlobal returns one page of the actor's visible merged feed with the
// total count of the same filter; the SQL visibility predicate is the
// adapter's (ticket #521, the merged visibility of ADR 0052 decision 3).
func (s *TaskStore) ListGlobal(
	ctx context.Context, actor uuid.UUID, q application.TasksListQuery,
) (rows []application.GlobalTaskRow, total int, err error) {
	return listGlobalPage(s.db, actor, q,
		func(queries *postgres.Queries) (int64, error) {
			return queries.CountTasksGlobal(ctx, postgres.CountTasksGlobalParams{
				OwnerID: pgconv.UUIDToPgtype(actor),
				Column2: q.Completed,
			})
		},
		func(queries *postgres.Queries, page postgres.ListActiveTasksGlobalParams) ([]postgres.ListActiveTasksGlobalRow, error) {
			return queries.ListActiveTasksGlobal(ctx, page)
		},
		func(queries *postgres.Queries, page postgres.ListCompletedTasksGlobalParams) ([]postgres.ListCompletedTasksGlobalRow, error) {
			return queries.ListCompletedTasksGlobal(ctx, page)
		},
		globalRowFields,
		globalCompletedRowFields,
	)
}

// ListGlobalWithoutProperty returns one page of the property-less cut of the
// actor's book with the total count of the same filter (ADR 0052).
func (s *TaskStore) ListGlobalWithoutProperty(
	ctx context.Context, actor uuid.UUID, q application.TasksListQuery,
) (rows []application.GlobalTaskRow, total int, err error) {
	return listGlobalPage(s.db, actor, q,
		func(queries *postgres.Queries) (int64, error) {
			return queries.CountTasksGlobalWithoutProperty(ctx, postgres.CountTasksGlobalWithoutPropertyParams{
				OwnerID: pgconv.UUIDToPgtype(actor),
				Column2: q.Completed,
			})
		},
		func(queries *postgres.Queries, page postgres.ListActiveTasksGlobalParams) ([]postgres.ListActiveTasksGlobalWithoutPropertyRow, error) {
			return queries.ListActiveTasksGlobalWithoutProperty(
				ctx, postgres.ListActiveTasksGlobalWithoutPropertyParams(page))
		},
		func(queries *postgres.Queries, page postgres.ListCompletedTasksGlobalParams) (
			[]postgres.ListCompletedTasksGlobalWithoutPropertyRow, error,
		) {
			return queries.ListCompletedTasksGlobalWithoutProperty(
				ctx, postgres.ListCompletedTasksGlobalWithoutPropertyParams(page))
		},
		withoutPropertyRowFields,
		withoutPropertyCompletedRowFields,
	)
}

// ListGlobalOfProperties returns one page of the listed properties' tasks
// with the total count of the same filter (ticket #547): the merged feed's
// SQL cut to property_id IN the list; the use case has already proven every
// id's visibility, archived properties contribute nothing (решение 9).
// The withoutProperty flag unions the actor's own property-less rows in —
// the feed filter's «Общие задачи» + objects (решение владельца 2026-09-07).
func (s *TaskStore) ListGlobalOfProperties(
	ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID, withoutProperty bool, q application.TasksListQuery,
) (rows []application.GlobalTaskRow, total int, err error) {
	ids := make([]string, len(propertyIDs))
	for i, id := range propertyIDs {
		ids[i] = id.String()
	}
	joined := strings.Join(ids, ",")
	return listGlobalPage(s.db, actor, q,
		func(queries *postgres.Queries) (int64, error) {
			return queries.CountTasksGlobalOfProperties(ctx, postgres.CountTasksGlobalOfPropertiesParams{
				OwnerID:         pgconv.UUIDToPgtype(actor),
				Column2:         q.Completed,
				PropertyIds:     joined,
				WithoutProperty: withoutProperty,
			})
		},
		func(queries *postgres.Queries, page postgres.ListActiveTasksGlobalParams) ([]postgres.ListActiveTasksGlobalOfPropertiesRow, error) {
			return queries.ListActiveTasksGlobalOfProperties(
				ctx, postgres.ListActiveTasksGlobalOfPropertiesParams{
					OwnerID:         page.OwnerID,
					Limit:           page.Limit,
					Offset:          page.Offset,
					PropertyIds:     joined,
					WithoutProperty: withoutProperty,
				})
		},
		func(queries *postgres.Queries, page postgres.ListCompletedTasksGlobalParams) (
			[]postgres.ListCompletedTasksGlobalOfPropertiesRow, error,
		) {
			return queries.ListCompletedTasksGlobalOfProperties(
				ctx, postgres.ListCompletedTasksGlobalOfPropertiesParams{
					OwnerID:         page.OwnerID,
					Limit:           page.Limit,
					Offset:          page.Offset,
					PropertyIds:     joined,
					WithoutProperty: withoutProperty,
				})
		},
		ofPropertiesRowFields,
		ofPropertiesCompletedRowFields,
	)
}

// listGlobalPage runs the global listings' shared shape: the bucket's total
// count, then one page of the requested bucket, mapped onto the application
// rows. The two cut queries differ only in their sqlc calls and the row
// projectors, so the dispatch lives here once. The bounds re-check the
// service applied (PrepareTasksQuery) guards the int→int32 narrowing.
func listGlobalPage[ActiveRow, CompletedRow any](
	db postgres.DBTX, actor uuid.UUID, q application.TasksListQuery,
	count func(*postgres.Queries) (int64, error),
	listActive func(*postgres.Queries, postgres.ListActiveTasksGlobalParams) ([]ActiveRow, error),
	listCompleted func(*postgres.Queries, postgres.ListCompletedTasksGlobalParams) ([]CompletedRow, error),
	projectActive func(ActiveRow) (taskFields, string),
	projectCompleted func(CompletedRow) (taskFields, string),
) ([]application.GlobalTaskRow, int, error) {
	if q.Limit < 1 || q.Limit > application.MaxTasksPageSize || q.Offset < 0 || q.Offset > math.MaxInt32 {
		return nil, 0, application.ErrInvalidInput
	}
	queries := postgres.New(db)
	counted, err := count(queries)
	if err != nil {
		return nil, 0, fmt.Errorf("count global tasks: %w", err)
	}
	page := postgres.ListActiveTasksGlobalParams{
		OwnerID: pgconv.UUIDToPgtype(actor),
		Limit:   int32(q.Limit),
		Offset:  int32(q.Offset),
	}
	var rows []application.GlobalTaskRow
	if q.Completed {
		fetched, listErr := listCompleted(queries, postgres.ListCompletedTasksGlobalParams(page))
		if listErr != nil {
			return nil, 0, fmt.Errorf("list completed global tasks: %w", listErr)
		}
		rows = mapGlobalRows(fetched, projectCompleted)
	} else {
		fetched, listErr := listActive(queries, page)
		if listErr != nil {
			return nil, 0, fmt.Errorf("list active global tasks: %w", listErr)
		}
		rows = mapGlobalRows(fetched, projectActive)
	}
	return rows, int(counted), nil
}

// mapGlobalRows projects one fetched page onto the application rows: the
// domain task per row plus the property label the query carried (the store
// fills Task and PropertyName; Status stays the use case's — the buckets are
// computed against each row owner's moment).
func mapGlobalRows[T any](rows []T, project func(T) (taskFields, string)) []application.GlobalTaskRow {
	items := make([]application.GlobalTaskRow, 0, len(rows))
	for _, row := range rows {
		fields, name := project(row)
		items = append(items, application.GlobalTaskRow{Task: mapTaskRow(fields), PropertyName: name})
	}
	return items
}

// globalRowFields reads the merged feed's row shape (the property label
// included).
func globalRowFields(row postgres.ListActiveTasksGlobalRow) (fields taskFields, propertyName string) {
	return taskFieldsFromRow(
			row.ID, row.OwnerID, row.PropertyID, row.RuleID, row.DueDate, row.DueTime,
			row.Title, row.Comment, row.CompletedDate, row.CreatedAt, row.UpdatedAt, row.RuleRepeat,
		),
		pgconv.TextToString(row.PropertyName)
}

// globalCompletedRowFields is globalRowFields for the completed journal's
// row type — the shapes are column-identical.
func globalCompletedRowFields(row postgres.ListCompletedTasksGlobalRow) (fields taskFields, propertyName string) {
	return globalRowFields(postgres.ListActiveTasksGlobalRow(row))
}

// ofPropertiesRowFields reads the listed-properties cut's row shape —
// column-identical to the merged feed's (ticket #547).
func ofPropertiesRowFields(
	row postgres.ListActiveTasksGlobalOfPropertiesRow,
) (fields taskFields, propertyName string) {
	return globalRowFields(postgres.ListActiveTasksGlobalRow(row))
}

// ofPropertiesCompletedRowFields is ofPropertiesRowFields for the completed
// journal's row type.
func ofPropertiesCompletedRowFields(
	row postgres.ListCompletedTasksGlobalOfPropertiesRow,
) (fields taskFields, propertyName string) {
	return globalRowFields(postgres.ListActiveTasksGlobalRow(row))
}

// withoutPropertyRowFields is the property-less cut's row shape; the label
// is always absent there.
func withoutPropertyRowFields(
	row postgres.ListActiveTasksGlobalWithoutPropertyRow,
) (fields taskFields, propertyName string) {
	return taskFieldsFromRow(
			row.ID, row.OwnerID, row.PropertyID, row.RuleID, row.DueDate, row.DueTime,
			row.Title, row.Comment, row.CompletedDate, row.CreatedAt, row.UpdatedAt, row.RuleRepeat,
		),
		""
}

// withoutPropertyCompletedRowFields is withoutPropertyRowFields for the
// completed journal's row type.
func withoutPropertyCompletedRowFields(
	row postgres.ListCompletedTasksGlobalWithoutPropertyRow,
) (fields taskFields, propertyName string) {
	return withoutPropertyRowFields(postgres.ListActiveTasksGlobalWithoutPropertyRow(row))
}

// taskFieldsFromRow bundles the shared column projection of the task
// readers.
func taskFieldsFromRow(
	id, ownerID, propertyID, ruleID pgtype.UUID,
	dueDate pgtype.Date, dueTime pgtype.Time,
	title string, comment pgtype.Text,
	completedDate pgtype.Date, createdAt, updatedAt pgtype.Timestamptz,
	ruleRepeat pgtype.Text,
) taskFields {
	return taskFields{
		ID:            pgconv.UUIDFromPgtype(id),
		OwnerID:       pgconv.UUIDFromPgtype(ownerID),
		PropertyID:    propertyID,
		RuleID:        ruleID,
		DueDate:       dueDate,
		DueTime:       dueTime,
		Title:         title,
		Comment:       comment,
		CompletedDate: completedDate,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		RuleRepeat:    ruleRepeat,
	}
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

// DeleteCompletedJournalOwnerBook removes the owner book's completed tasks
// of deleted rules across both slices in one query and reports the count
// (ticket #536).
func (s *TaskStore) DeleteCompletedJournalOwnerBook(ctx context.Context, scope uuid.UUID) (int64, error) {
	cleared, err := s.q().DeleteCompletedJournalOwnerBook(ctx, pgconv.UUIDToPgtype(scope))
	if err != nil {
		return 0, fmt.Errorf("delete completed journal of owner %s: %w", scope, err)
	}
	return cleared, nil
}
