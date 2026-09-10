package postgres

// OperationStore is the postgres adapter of the operations port (ticket #461,
// ADR 0049 §4). Reads and writes are scoped by the data owner and the nested
// property→operation path lives in the queries; the mutating methods run
// inside the transaction holding the property serialization lock. The two
// listing scopes share one SQL query behind the port — a NULL payment widens
// it from one rule to the whole property.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.OperationStore = (*OperationStore)(nil)

// OperationStore persists and lists operations.
type OperationStore struct {
	db postgres.DBTX
}

// NewOperationStore creates an operation store over the given connection or
// pool.
func NewOperationStore(db postgres.DBTX) *OperationStore {
	return &OperationStore{db: db}
}

func (s *OperationStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *OperationStore) WithTx(tx transaction.Tx) (application.OperationStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.OperationStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewOperationStore(dbtx), nil
}

// Get loads one operation; pgx.ErrNoRows — an unknown id, another owner's row
// or another property's row — becomes the application ErrNotFound.
func (s *OperationStore) Get(
	ctx context.Context, id, scope, propertyID uuid.UUID,
) (domain.Operation, error) {
	row, err := s.q().GetOperationByID(ctx, postgres.GetOperationByIDParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operation{}, application.ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation %s: %w", id, err)
	}
	return mapOperationRow(operationRowFieldsFromGet(row)), nil
}

// MarkPaid flips the still-planned operation to paid with the given date;
// rows affected = 0 — a repeated pay under the same serialization lock —
// surfaces as ErrAlreadyPaid.
func (s *OperationStore) MarkPaid(ctx context.Context, id, scope uuid.UUID, paidDate time.Time) error {
	affected, err := s.q().PayOperationByID(ctx, postgres.PayOperationByIDParams{
		ID:       pgconv.UUIDToPgtype(id),
		OwnerID:  pgconv.UUIDToPgtype(scope),
		PaidDate: pgconv.DateToPgtype(paidDate),
	})
	if err != nil {
		return fmt.Errorf("pay operation %s: %w", id, err)
	}
	if affected == 0 {
		return application.ErrAlreadyPaid
	}
	return nil
}

// Cancel flips a planned or paid operation to the cancelled tombstone and
// clears paid_date with the payment fact; the row stays and keeps its
// (payment_id, date) key so the tick never re-materializes the occurrence.
// Rows affected = 0 — an unknown id, a foreign row or an already-cancelled
// one — surfaces as ErrNotFound: cancelled operations are gone for every
// read (lists exclude the status, the single GET reports not-found).
func (s *OperationStore) Cancel(ctx context.Context, id, scope, propertyID uuid.UUID) error {
	affected, err := s.q().CancelOperationByID(ctx, postgres.CancelOperationByIDParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return fmt.Errorf("cancel operation %s: %w", id, err)
	}
	if affected == 0 {
		return application.ErrNotFound
	}
	return nil
}

// Create inserts the manual one-off operation (ticket #569). The born-paid
// invariants — origin='manual', status='paid', date=paid_date, the NULL
// payment link and the NULL payment form — are the query's, not the
// caller's: the manual fact cannot be inserted in any other shape.
func (s *OperationStore) Create(ctx context.Context, op domain.Operation) error {
	if err := s.q().CreateManualOperation(ctx, postgres.CreateManualOperationParams{
		ID:            pgconv.UUIDToPgtype(op.ID),
		OwnerID:       pgconv.UUIDToPgtype(op.OwnerID),
		PropertyID:    pgconv.UUIDToPgtype(op.PropertyID),
		Date:          pgconv.DateToPgtype(op.Date),
		Type:          string(op.Type),
		Title:         op.Title,
		AmountKopecks: op.AmountKopecks,
		CategoryLabel: op.CategoryLabel,
		CategorySlug:  pgconv.StringPtrToPgtype(op.CategorySlug),
	}); err != nil {
		return fmt.Errorf("create manual operation %s: %w", op.ID, err)
	}
	return nil
}

// ListByPayment returns one rule's operations in the query's order.
func (s *OperationStore) ListByPayment(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID, q application.OperationsListQuery,
) ([]domain.Operation, error) {
	rows, err := s.q().ListOperations(ctx, listOperationsParams(postgres.ListOperationsParams{
		Owner:    pgconv.UUIDToPgtype(scope),
		Property: pgconv.UUIDToPgtype(propertyID),
		Payment:  pgconv.UUIDToPgtype(paymentID),
	}, q))
	if err != nil {
		return nil, fmt.Errorf("list operations of payment %s: %w", paymentID, err)
	}
	return mapOperationRows(rows), nil
}

// ListByProperty returns the property's operations across its rules; the
// payment filter stays NULL so every rule's rows come through.
func (s *OperationStore) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID, q application.OperationsListQuery,
) ([]domain.Operation, error) {
	rows, err := s.q().ListOperations(ctx, listOperationsParams(postgres.ListOperationsParams{
		Owner:    pgconv.UUIDToPgtype(scope),
		Property: pgconv.UUIDToPgtype(propertyID),
	}, q))
	if err != nil {
		return nil, fmt.Errorf("list operations of property %s: %w", propertyID, err)
	}
	return mapOperationRows(rows), nil
}

// CountPaidOperationsByPayment counts one rule's paid operations (ADR 0053
// §2: the rentals progress' paidMonths). The query counts in SQL — the
// count never rides a paginated listing.
func (s *OperationStore) CountPaidOperationsByPayment(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID,
) (int64, error) {
	count, err := s.q().CountPaidOperationsByPayment(ctx, postgres.CountPaidOperationsByPaymentParams{
		Owner:    pgconv.UUIDToPgtype(scope),
		Property: pgconv.UUIDToPgtype(propertyID),
		Payment:  pgconv.UUIDToPgtype(paymentID),
	})
	if err != nil {
		return 0, fmt.Errorf("count paid operations of payment %s: %w", paymentID, err)
	}
	return count, nil
}

// ListGlobal returns one page of the actor's visible paid operations — the
// merged feed (ticket #540); the visibility predicate and the archive cut
// are the query's. The bounds re-check the service applied
// (PrepareGlobalOperationsQuery) guards the int→int32 narrowing.
func (s *OperationStore) ListGlobal(
	ctx context.Context, actor uuid.UUID, q application.GlobalOperationsListQuery,
) ([]application.GlobalOperationRow, error) {
	if q.Limit < 1 || q.Limit > application.MaxOperationsPageSize || q.Offset < 0 {
		return nil, application.ErrInvalidInput
	}
	rows, err := s.q().ListPaidOperationsGlobal(ctx, postgres.ListPaidOperationsGlobalParams{
		Actor:           pgconv.UUIDToPgtype(actor),
		PropertyIds:     joinPropertyIDs(q.PropertyIDs),
		DateFrom:        pgconv.DatePtrToPgtype(q.DateFrom),
		DateTo:          pgconv.DatePtrToPgtype(q.DateTo),
		Search:          escapeLikePattern(q.Search),
		SearchDigits:    searchAmountDigits(q.Search),
		Type:            operationsTypeFilter(q.Type),
		Categories:      joinCategorySlugs(q.Categories),
		IncludeArchived: q.IncludeArchived,
		Order:           operationsOrder(q.Asc),
		Offset:          paginationToInt32(q.Offset),
		Limit:           paginationToInt32(q.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list global operations: %w", err)
	}
	out := make([]application.GlobalOperationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, application.GlobalOperationRow{
			Operation: mapOperationRow(operationRowFields{
				ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID,
				PaymentID: row.PaymentID, Origin: row.Origin, Date: row.Date,
				PaidDate: row.PaidDate, Status: row.Status, Type: row.Type,
				Title: row.Title, AmountKopecks: row.AmountKopecks,
				PaymentForm: row.PaymentForm, CategoryLabel: row.CategoryLabel,
				CategorySlug: row.CategorySlug,
			}),
			PropertyName: row.PropertyName,
		})
	}
	return out, nil
}

// SummarizeGlobal runs the global summary's two aggregations (ticket #540)
// over the actor's visible paid operations: the period totals by direction
// and the per-category breakdown. The totals carry no direction or category
// filter — the contract reports both directions whatever the breakdown is
// narrowed to. The two reads run as plain statements, like every listing
// read.
func (s *OperationStore) SummarizeGlobal(
	ctx context.Context, actor uuid.UUID, q application.GlobalOperationsSummaryQuery,
) (application.OperationsSummary, error) {
	search := escapeLikePattern(q.Search)
	searchDigits := searchAmountDigits(q.Search)
	propertyIDs := joinPropertyIDs(q.PropertyIDs)
	totalsParams := postgres.SumPaidOperationTotalsGlobalParams{
		Actor:           pgconv.UUIDToPgtype(actor),
		PropertyIds:     propertyIDs,
		DateFrom:        pgconv.DatePtrToPgtype(q.DateFrom),
		DateTo:          pgconv.DatePtrToPgtype(q.DateTo),
		Search:          search,
		SearchDigits:    searchDigits,
		IncludeArchived: q.IncludeArchived,
	}
	totals, err := s.q().SumPaidOperationTotalsGlobal(ctx, totalsParams)
	if err != nil {
		return application.OperationsSummary{}, fmt.Errorf("sum global operation totals: %w", err)
	}

	categories, err := s.q().SumPaidOperationsByCategoryGlobal(ctx, postgres.SumPaidOperationsByCategoryGlobalParams{
		Actor:           totalsParams.Actor,
		PropertyIds:     totalsParams.PropertyIds,
		DateFrom:        totalsParams.DateFrom,
		DateTo:          totalsParams.DateTo,
		Search:          search,
		SearchDigits:    searchDigits,
		Type:            operationsTypeFilter(q.Type),
		Categories:      joinCategorySlugs(q.Categories),
		IncludeArchived: q.IncludeArchived,
	})
	if err != nil {
		return application.OperationsSummary{}, fmt.Errorf("sum global operations by category: %w", err)
	}

	summary := application.OperationsSummary{
		Categories: make([]application.CategorySummary, 0, len(categories)),
	}
	for _, row := range totals {
		switch domain.PaymentType(row.Type) {
		case domain.TypeIncome:
			summary.IncomeTotalKopecks = row.TotalKopecks
		case domain.TypeExpense:
			summary.ExpenseTotalKopecks = row.TotalKopecks
		}
	}
	for _, row := range categories {
		summary.Categories = append(summary.Categories, application.CategorySummary{
			Slug:         row.CategorySlug.String,
			Label:        row.CategoryLabel,
			Type:         domain.PaymentType(row.Type),
			TotalKopecks: row.TotalKopecks,
		})
	}
	return summary, nil
}

// listOperationsParams folds the normalized query into the merged SQL
// parameters; the pagination width clamp and the direction/status encodings
// are this adapter's business.
func listOperationsParams(
	params postgres.ListOperationsParams, q application.OperationsListQuery,
) postgres.ListOperationsParams {
	params.Status = operationsStatusFilter(q.Status)
	params.Today = pgconv.DateToPgtype(q.Today)
	params.DateFrom = pgconv.DatePtrToPgtype(q.DateFrom)
	params.DateTo = pgconv.DatePtrToPgtype(q.DateTo)
	params.Search = escapeLikePattern(q.Search)
	params.SearchDigits = searchAmountDigits(q.Search)
	params.Type = operationsTypeFilter(q.Type)
	params.Categories = joinCategorySlugs(q.Categories)
	params.Order = operationsOrder(q.Asc)
	params.Offset = paginationToInt32(q.Offset)
	params.Limit = paginationToInt32(q.Limit)
	return params
}

// SummarizeByProperty runs the summary's two aggregations (ticket #473):
// the period totals by direction and the per-category breakdown. The store
// reports absent directions as zero totals; the breakdown arrives from SQL
// already ordered by total, largest first. The two reads run as plain
// statements — like every listing read, they accept a mid-summary mutation
// racing one statement against another; the summary cards tolerate that.
func (s *OperationStore) SummarizeByProperty(
	ctx context.Context, scope, propertyID uuid.UUID, q application.OperationsSummaryQuery,
) (application.OperationsSummary, error) {
	// The totals deliberately carry no direction filter — the contract
	// reports both directions whatever the categories are narrowed to.
	search := escapeLikePattern(q.Search)
	searchDigits := searchAmountDigits(q.Search)
	params := postgres.SumOperationTotalsParams{
		Owner:        pgconv.UUIDToPgtype(scope),
		Property:     pgconv.UUIDToPgtype(propertyID),
		Status:       operationsStatusFilter(q.Status),
		Today:        pgconv.DateToPgtype(q.Today),
		DateFrom:     pgconv.DatePtrToPgtype(q.DateFrom),
		DateTo:       pgconv.DatePtrToPgtype(q.DateTo),
		Search:       search,
		SearchDigits: searchDigits,
	}
	totals, err := s.q().SumOperationTotals(ctx, params)
	if err != nil {
		return application.OperationsSummary{}, fmt.Errorf("sum operation totals of property %s: %w", propertyID, err)
	}

	categories, err := s.q().SumOperationsByCategory(ctx, postgres.SumOperationsByCategoryParams{
		Owner:        params.Owner,
		Property:     params.Property,
		Status:       params.Status,
		Today:        params.Today,
		DateFrom:     params.DateFrom,
		DateTo:       params.DateTo,
		Search:       search,
		SearchDigits: searchDigits,
		Type:         operationsTypeFilter(q.Type),
	})
	if err != nil {
		return application.OperationsSummary{}, fmt.Errorf("sum operations by category of property %s: %w", propertyID, err)
	}

	summary := application.OperationsSummary{
		Categories: make([]application.CategorySummary, 0, len(categories)),
	}
	for _, row := range totals {
		switch domain.PaymentType(row.Type) {
		case domain.TypeIncome:
			summary.IncomeTotalKopecks = row.TotalKopecks
		case domain.TypeExpense:
			summary.ExpenseTotalKopecks = row.TotalKopecks
		}
	}
	for _, row := range categories {
		summary.Categories = append(summary.Categories, application.CategorySummary{
			Slug:         row.CategorySlug.String,
			Label:        row.CategoryLabel,
			Type:         domain.PaymentType(row.Type),
			TotalKopecks: row.TotalKopecks,
		})
	}
	return summary, nil
}

// mapOperationRows projects listed rows onto the domain shape.
func mapOperationRows(rows []postgres.ListOperationsRow) []domain.Operation {
	out := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperationRow(operationRowFields{
			ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID,
			PaymentID: row.PaymentID, Origin: row.Origin, Date: row.Date,
			PaidDate: row.PaidDate, Status: row.Status, Type: row.Type,
			Title: row.Title, AmountKopecks: row.AmountKopecks,
			PaymentForm: row.PaymentForm, CategoryLabel: row.CategoryLabel,
			CategorySlug: row.CategorySlug,
		}))
	}
	return out
}

// operationsStatusFilter encodes the view status filter for the SQL: nil is
// any status; planned/paid/overdue split against the query's today inside the
// WHERE clause (ticket #461).
func operationsStatusFilter(status *domain.OperationViewStatus) string {
	if status == nil {
		return ""
	}
	return string(*status)
}

// operationsTypeFilter encodes the direction filter for the SQL: nil is any
// direction; income/expense compare against the operation's type snapshot.
func operationsTypeFilter(typ *domain.PaymentType) string {
	if typ == nil {
		return ""
	}
	return string(*typ)
}

// joinCategorySlugs encodes the category filter for the SQL's
// string_to_array split: ” is any category. Operations snapshot only the
// default catalog's slug — kebab-case, never a comma; user-category rows
// carry a NULL slug, which matches no filter value by design.
func joinCategorySlugs(slugs []string) string {
	return strings.Join(slugs, ",")
}

// joinPropertyIDs encodes the propertyIds multi-select for the SQL's
// string_to_array→uuid[] cast: nil is any property — the merged feed. UUIDs
// never carry commas, so the join round-trips; the view gate has already
// vetted every entry before the store runs.
func joinPropertyIDs(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id.String())
	}
	return strings.Join(parts, ",")
}

// operationsOrder encodes the sort direction ('asc' | 'desc'); false encodes
// the contract default — desc, newest first.
func operationsOrder(asc bool) string {
	if asc {
		return "asc"
	}
	return "desc"
}

// amountQuerySeparators are the characters besides digits a query may carry
// and still count as an amount query: the thousand separators and the decimal
// comma/point of the display format and the minus of a negative amount —
// regular, nbsp and narrow-nbsp spaces; comma and point; the minus family
// (hyphen, figure dash, en/em dash, minus sign).
var amountQuerySeparators = func() map[rune]bool {
	separators := map[rune]bool{}
	for _, r := range []rune{
		' ', '\u00a0', '\u202f',
		',', '.',
		'-', '\u2010', '\u2012', '\u2013', '\u2014', '\u2212',
	} {
		separators[r] = true
	}
	return separators
}()

// searchAmountDigits extracts the digits of a query that reads as an amount
// (digits plus amount separators only, at least one digit): the SQL searches
// them inside the operation amount's decimal digits in kopecks — the display
// amount without separators, so «2 500» finds 2 500,00 ₽ and «2500,50» finds
// 2 500,50 ₽ (ticket #476). Any letter switches the amount match off — a
// word with a stray digit must not widen the search onto every amount. An
// empty result disables the amount clause on the SQL side.
func searchAmountDigits(q string) string {
	hasDigit := false
	var digits strings.Builder
	for _, r := range strings.TrimSpace(q) {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
			digits.WriteRune(r)
		case !amountQuerySeparators[r]:
			return ""
		}
	}
	if !hasDigit {
		return ""
	}
	return digits.String()
}

// paginationToInt32 narrows the validated pagination values onto SQL's LIMIT/
// OFFSET width. PrepareOperationsQuery has already bounded both at the
// application boundary; the clamp keeps this adapter total regardless.
func paginationToInt32(v int) int32 {
	const maxInt32 = int64(^uint32(0) >> 1)
	switch {
	case v < 0:
		return 0
	case int64(v) > maxInt32:
		return int32(maxInt32)
	default:
		return int32(v)
	}
}
