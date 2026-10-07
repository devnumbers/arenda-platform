package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// Shared wire fixtures of the handler tests.
const (
	testSlugRent     = "rent"
	testLabelRent    = "Арендная плата"
	testDetailNotFmt = "Не найдено"
	testTitleRent    = "Аренда"
	// TestSlugUtilities is the utilities slug of the operation fixtures
	// (the category snapshot the manual-creation tests freeze).
	testSlugUtilities = "utilities"
)

// fakePaymentManager is the func-backed PaymentManager double: every use case
// is optional; an unset one fails the test loudly instead of silently
// succeeding (ADR 0035 test doubles). The func fields back the port's
// methods one to one; the field order is alphabetical, unlike the port.
type fakePaymentManager struct {
	create   func(ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreatePaymentCommand) (domain.Payment, error)
	del      func(ctx context.Context, actor, propertyID, paymentID uuid.UUID, keepOverdue bool) error
	favorite func(ctx context.Context, actor, propertyID, paymentID uuid.UUID, favorite bool) (domain.Payment, error)
	get      func(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error)
	list     func(ctx context.Context, actor, propertyID uuid.UUID, search string) ([]application.PaymentListItem, error)
	pause    func(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error)
	resume   func(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error)
	update   func(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd application.UpdatePaymentCommand,
	) (domain.Payment, error)
	completedStatus func(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (bool, error)
	paymentFlags    func(ctx context.Context, actor, propertyID uuid.UUID) (
		completed, managed, rentalCompleted map[uuid.UUID]bool, err error)
	rentalManagedStatus func(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (bool, error)
	rentalCompleted     func(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (bool, error)
	paymentChanges      func(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID, q application.ChangesQuery,
	) (application.ChangeLogPage, error)
}

// CompletedStatus defaults to false — the pre-completion behaviour the older
// wire assertions expect unless a test sets the hook.
func (f *fakePaymentManager) CompletedStatus(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (bool, error) {
	if f.completedStatus == nil {
		return false, nil
	}
	return f.completedStatus(ctx, actor, propertyID, paymentID)
}

// PaymentFlags defaults to three empty maps — no listed rule carries any
// flag unless a test sets the hook.
func (f *fakePaymentManager) PaymentFlags(
	ctx context.Context, actor, propertyID uuid.UUID,
) (completed, managed, rentalCompleted map[uuid.UUID]bool, err error) {
	if f.paymentFlags == nil {
		return map[uuid.UUID]bool{}, map[uuid.UUID]bool{}, map[uuid.UUID]bool{}, nil
	}
	return f.paymentFlags(ctx, actor, propertyID)
}

// RentalManagedStatus defaults to false — the ordinary (unmanaged) rule the
// older wire assertions expect unless a test sets the hook.
func (f *fakePaymentManager) RentalManagedStatus(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (bool, error) {
	if f.rentalManagedStatus == nil {
		return false, nil
	}
	return f.rentalManagedStatus(ctx, actor, propertyID, paymentID)
}

// RentalCompletedStatus defaults to false — the managing rental is
// unfinished unless a test sets the hook.
func (f *fakePaymentManager) RentalCompletedStatus(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
) (bool, error) {
	if f.rentalCompleted == nil {
		return false, nil
	}
	return f.rentalCompleted(ctx, actor, propertyID, paymentID)
}

// PaymentChanges defaults to an empty page — the change log read only runs
// when a test sets the hook.
func (f *fakePaymentManager) PaymentChanges(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, q application.ChangesQuery,
) (application.ChangeLogPage, error) {
	if f.paymentChanges == nil {
		return application.ChangeLogPage{Items: []domain.ChangeEntry{}}, nil
	}
	return f.paymentChanges(ctx, actor, propertyID, paymentID, q)
}

// The method set mirrors the port; the long signatures are the contract's.

func (f *fakePaymentManager) CreatePayment(
	ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreatePaymentCommand,
) (domain.Payment, error) {
	if f.create == nil {
		return domain.Payment{}, errors.New("unexpected CreatePayment call")
	}
	return f.create(ctx, actor, propertyID, cmd)
}

func (f *fakePaymentManager) ListPayments(
	ctx context.Context, actor, propertyID uuid.UUID, search string,
) ([]application.PaymentListItem, error) {
	if f.list == nil {
		return nil, errors.New("unexpected ListPayments call")
	}
	return f.list(ctx, actor, propertyID, search)
}

func (f *fakePaymentManager) GetPayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error) {
	if f.get == nil {
		return domain.Payment{}, errors.New("unexpected GetPayment call")
	}
	return f.get(ctx, actor, propertyID, paymentID)
}

func (f *fakePaymentManager) UpdatePayment(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd application.UpdatePaymentCommand,
) (domain.Payment, error) {
	if f.update == nil {
		return domain.Payment{}, errors.New("unexpected UpdatePayment call")
	}
	return f.update(ctx, actor, propertyID, paymentID, cmd)
}

func (f *fakePaymentManager) DeletePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID, keepOverdue bool) error {
	if f.del == nil {
		return errors.New("unexpected DeletePayment call")
	}
	return f.del(ctx, actor, propertyID, paymentID, keepOverdue)
}

func (f *fakePaymentManager) PausePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error) {
	if f.pause == nil {
		return domain.Payment{}, errors.New("unexpected PausePayment call")
	}
	return f.pause(ctx, actor, propertyID, paymentID)
}

func (f *fakePaymentManager) ResumePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error) {
	if f.resume == nil {
		return domain.Payment{}, errors.New("unexpected ResumePayment call")
	}
	return f.resume(ctx, actor, propertyID, paymentID)
}

func (f *fakePaymentManager) SetPaymentFavorite(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID, favorite bool,
) (domain.Payment, error) {
	if f.favorite == nil {
		return domain.Payment{}, errors.New("unexpected SetPaymentFavorite call")
	}
	return f.favorite(ctx, actor, propertyID, paymentID, favorite)
}

// paymentRequest builds an authenticated request against the payments
// endpoints with an optional JSON body.
func paymentRequest(t *testing.T, method string, userID uuid.UUID, body string) *http.Request {
	t.Helper()
	reader := strings.NewReader(body)
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), userID),
		method, "/payments", reader,
	)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// validCreateBody is a well-formed create payload: weekly rent.
const validCreateBody = `{
	"type": "expense",
	"title": "Арендная плата",
	"amountKopecks": 5000000,
	"recurrence": {"kind": "weekly", "weekdays": [1, 3]},
	"categorySlug": "rent",
	"endDate": "2027-01-31",
	"autoPay": false
}`

func TestPaymentHandlers_RequireAuth(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandlers(&fakePaymentManager{}, nil)
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name string
		call func(w http.ResponseWriter, r *http.Request)
	}{
		{"create", func(w http.ResponseWriter, r *http.Request) {
			h.CreatePayment(w, r, propertyID)
		}},
		{"list", func(w http.ResponseWriter, r *http.Request) {
			h.ListPayments(w, r, propertyID, openapi.ListPaymentsParams{})
		}},
		{"get", func(w http.ResponseWriter, r *http.Request) {
			h.GetPayment(w, r, propertyID, paymentID)
		}},
		{"update", func(w http.ResponseWriter, r *http.Request) {
			h.UpdatePayment(w, r, propertyID, paymentID)
		}},
		{"delete", func(w http.ResponseWriter, r *http.Request) {
			h.DeletePayment(w, r, propertyID, paymentID, openapi.DeletePaymentParams{})
		}},
		{"pause", func(w http.ResponseWriter, r *http.Request) {
			h.PausePayment(w, r, propertyID, paymentID)
		}},
		{"resume", func(w http.ResponseWriter, r *http.Request) {
			h.ResumePayment(w, r, propertyID, paymentID)
		}},
		{"favorite", func(w http.ResponseWriter, r *http.Request) {
			h.SetPaymentFavorite(w, r, propertyID, paymentID)
		}},
		{"changes", func(w http.ResponseWriter, r *http.Request) {
			h.ListPaymentChanges(w, r, propertyID, paymentID, openapi.ListPaymentChangesParams{})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			tc.call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
		})
	}
}

func TestCreatePayment_RejectsMalformedBodies(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandlers(&fakePaymentManager{}, nil)
	actor := uuid.Must(uuid.NewV7())

	cases := []struct {
		name string
		body string
	}{
		{"broken json", `{"type": "expense"`},
		{"unknown field", `{"type": "expense", "since": "2026-01-01"}`},
		// «Форма оплаты» снесена из контракта (карта #1005, тикеты #1006–#1009):
		// миграционный шим декодера снят — поле строго отклоняется; тело
		// валидно, чтобы причиной 400 было только неизвестное поле.
		{
			"removed paymentForm field",
			`{"type":"expense","title":"Аренда","amountKopecks":5000000,` +
				`"recurrence":{"kind":"daily"},"categorySlug":"rent","paymentForm":"cash"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			h.CreatePayment(w, paymentRequest(t, http.MethodPost, actor, tc.body),
				uuid.Must(uuid.NewV7()))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// The rule-level rejections (enums, amount bounds, title length, catalog
// slug, endDate before since) live in the payment rule module and are
// covered by the application integration tests; these transport cases cover
// what the wire decode and the recurrence construction reject.
func TestCreatePayment_RejectsInvalidBodies(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandlers(&fakePaymentManager{}, nil)
	actor := uuid.Must(uuid.NewV7())

	mutate := func(pairs ...string) string {
		body := map[string]any{
			"type":          "expense",
			"title":         "Аренда",
			"amountKopecks": 500000,
			"recurrence":    map[string]any{"kind": "monthly", "dayOfMonth": 5},
			"categorySlug":  "rent",
		}
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i+1] == "<delete>" {
				delete(body, pairs[i])
				continue
			}
			var value any
			if err := json.Unmarshal([]byte(pairs[i+1]), &value); err != nil {
				t.Fatalf("fixture %s: %v", pairs[i], err)
			}
			body[pairs[i]] = value
		}
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		return string(raw)
	}

	cases := []struct {
		name string
		body string
	}{
		{"amount not an integer", mutate("amountKopecks", `500.5`)},
		{"bad endDate format", mutate("endDate", `"31.01.2027"`)},
		{"recurrence unknown kind", mutate("recurrence", `{"kind":"hourly"}`)},
		{"recurrence weekly empty", mutate("recurrence", `{"kind":"weekly","weekdays":[]}`)},
		{"recurrence weekly duplicate days", mutate("recurrence", `{"kind":"weekly","weekdays":[1,1]}`)},
		{"recurrence weekly day out of range", mutate("recurrence", `{"kind":"weekly","weekdays":[7]}`)},
		{"recurrence monthly zero day", mutate("recurrence", `{"kind":"monthly","dayOfMonth":0}`)},
		{"recurrence monthly day 32", mutate("recurrence", `{"kind":"monthly","dayOfMonth":32}`)},
		{"recurrence monthly day missing", mutate("recurrence", `{"kind":"monthly"}`)},
		{"recurrence yearly month 13", mutate("recurrence", `{"kind":"yearly","month":13,"day":1}`)},
		{"recurrence yearly day 32", mutate("recurrence", `{"kind":"yearly","month":5,"day":32}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			h.CreatePayment(w, paymentRequest(t, http.MethodPost, actor, tc.body),
				uuid.Must(uuid.NewV7()))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreatePayment_MapsCommandToService(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	since := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	slug := testSlugRent

	respond := domain.Payment{
		ID: paymentID, OwnerID: actor, PropertyID: propertyID,
		Type: domain.TypeExpense, Title: testTitleRent, AmountKopecks: 5000000,
		Recurrence: mustWeekly(t, 1, 3),
		Since:      since, EndDate: &endDate, AutoPay: false,
		Category:  domain.CategoryRef{Slug: &slug},
		Pauses:    []domain.PauseInterval{{From: since}, {From: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), To: &to}},
		CreatedAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
	}
	var gotCmd application.CreatePaymentCommand
	h := NewPaymentHandlers(&fakePaymentManager{
		create: func(_ context.Context, _, _ uuid.UUID, cmd application.CreatePaymentCommand) (domain.Payment, error) {
			gotCmd = cmd
			return respond, nil
		},
	}, nil)

	w := httptest.NewRecorder()
	h.CreatePayment(w, paymentRequest(t, http.MethodPost, actor, validCreateBody),
		propertyID)

	// The command reaches the application layer with the parsed wire payload.
	if gotCmd.Type != domain.TypeExpense || gotCmd.Title != testLabelRent || gotCmd.AmountKopecks != 5000000 {
		t.Fatalf("command scalars = %+v", gotCmd)
	}
	if gotCmd.Recurrence.Kind() != domain.RecurrenceWeekly {
		t.Fatalf("command recurrence kind = %q, want weekly", gotCmd.Recurrence.Kind())
	}
	if len(gotCmd.Recurrence.Weekdays()) != 2 || gotCmd.Recurrence.Weekdays()[0] != time.Monday {
		t.Fatalf("command weekdays = %v, want [Monday Wednesday]", gotCmd.Recurrence.Weekdays())
	}
	if gotCmd.CategorySlug != testSlugRent {
		t.Fatalf("command category = %q", gotCmd.CategorySlug)
	}
	if gotCmd.EndDate == nil || gotCmd.EndDate.Format(time.DateOnly) != "2027-01-31" {
		t.Fatalf("command endDate = %v, want 2027-01-31", gotCmd.EndDate)
	}

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestCreatePayment_MapsResponseContract(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	since := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	slug := testSlugRent

	respond := domain.Payment{
		ID: paymentID, OwnerID: actor, PropertyID: propertyID,
		Type: domain.TypeExpense, Title: testTitleRent, AmountKopecks: 5000000,
		Recurrence: mustWeekly(t, 1, 3),
		Since:      since, EndDate: &endDate, AutoPay: false,
		Category:  domain.CategoryRef{Slug: &slug},
		Pauses:    []domain.PauseInterval{{From: since}, {From: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), To: &to}},
		CreatedAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
	}
	h := NewPaymentHandlers(&fakePaymentManager{
		create: func(context.Context, uuid.UUID, uuid.UUID, application.CreatePaymentCommand) (domain.Payment, error) {
			return respond, nil
		},
	}, nil)

	w := httptest.NewRecorder()
	h.CreatePayment(w, paymentRequest(t, http.MethodPost, actor, validCreateBody), propertyID)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID            string         `json:"id"`
		PropertyID    string         `json:"propertyId"`
		Type          string         `json:"type"`
		Title         string         `json:"title"`
		AmountKopecks int64          `json:"amountKopecks"`
		Recurrence    map[string]any `json:"recurrence"`
		Since         string         `json:"since"`
		EndDate       *string        `json:"endDate"`
		AutoPay       bool           `json:"autoPay"`
		Category      struct {
			Source string  `json:"source"`
			Slug   *string `json:"slug"`
			ID     *string `json:"id"`
			Label  string  `json:"label"`
		} `json:"category"`
		Pauses []struct {
			FromDate string  `json:"fromDate"`
			ToDate   *string `json:"toDate"`
		} `json:"pauses"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != paymentID.String() || resp.PropertyID != propertyID.String() {
		t.Fatalf("ids = %s/%s", resp.ID, resp.PropertyID)
	}
	if resp.Recurrence["kind"] != "weekly" {
		t.Fatalf("recurrence = %v", resp.Recurrence)
	}
	if resp.Since != "2026-08-25" || resp.EndDate == nil || *resp.EndDate != "2027-01-31" {
		t.Fatalf("since/endDate = %q/%v", resp.Since, resp.EndDate)
	}
	if resp.Category.Source != "default" || resp.Category.Slug == nil || *resp.Category.Slug != testSlugRent {
		t.Fatalf("category = %+v", resp.Category)
	}
	if resp.Category.Label != testLabelRent {
		t.Fatalf("category label = %q, want the catalog label", resp.Category.Label)
	}
	assertPauseViews(t, resp.Pauses)
}

// assertPauseViews checks the wire shape of the response's pause intervals.
func assertPauseViews(t *testing.T, pauses []struct {
	FromDate string  `json:"fromDate"`
	ToDate   *string `json:"toDate"`
},
) {
	t.Helper()
	if len(pauses) != 2 {
		t.Fatalf("pauses = %d, want 2", len(pauses))
	}
	if pauses[0].FromDate != "2026-08-25" || pauses[0].ToDate != nil {
		t.Fatalf("active pause = %+v, want open-ended from 2026-08-25", pauses[0])
	}
	if pauses[1].ToDate == nil || *pauses[1].ToDate != "2026-09-01" {
		t.Fatalf("closed pause = %+v, want to 2026-09-01", pauses[1])
	}
}

func TestCreatePayment_CategoryViewFallbacksAndCustom(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	removedSlug := "removed-from-catalog"
	userCategoryID := uuid.Must(uuid.NewV7())
	userCategoryName := "Моя категория"

	cases := []struct {
		name       string
		category   domain.CategoryRef
		wantSource string
		wantLabel  string
	}{
		{
			"slug removed from catalog falls back to Прочее",
			domain.CategoryRef{Slug: &removedSlug},
			"default", "Прочее",
		},
		{
			"user category resolves to custom",
			domain.CategoryRef{UserCategoryID: &userCategoryID, UserCategoryName: &userCategoryName},
			"custom", "Моя категория",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewPaymentHandlers(&fakePaymentManager{
				create: func(context.Context, uuid.UUID, uuid.UUID, application.CreatePaymentCommand) (domain.Payment, error) {
					payment := validFixturePayment()
					payment.Category = tc.category
					return payment, nil
				},
			}, nil)
			w := httptest.NewRecorder()
			h.CreatePayment(w, paymentRequest(t, http.MethodPost, actor, validCreateBody),
				uuid.Must(uuid.NewV7()))
			var resp struct {
				Category struct {
					Source string  `json:"source"`
					Slug   *string `json:"slug"`
					ID     *string `json:"id"`
					Label  string  `json:"label"`
				} `json:"category"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Category.Source != tc.wantSource || resp.Category.Label != tc.wantLabel {
				t.Fatalf("category = %+v, want source=%s label=%q", resp.Category, tc.wantSource, tc.wantLabel)
			}
		})
	}
}

func TestUpdatePayment_EndDateTriState(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())

	// Table fields: wantSet — the command carries an EndDateUpdate; wantClr —
	// the update clears the end date; wantDay — the set date, empty otherwise.
	cases := []struct {
		name    string
		body    string
		wantSet bool
		wantClr bool
		wantDay string
	}{
		{"omitted keeps", `{"title": "Новое название"}`, false, false, ""},
		{"null clears", `{"endDate": null}`, true, true, ""},
		{"date sets", `{"endDate": "2027-03-01"}`, true, false, "2027-03-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotCmd application.UpdatePaymentCommand
			h := NewPaymentHandlers(&fakePaymentManager{
				update: func(_ context.Context, _, _, _ uuid.UUID, cmd application.UpdatePaymentCommand) (domain.Payment, error) {
					gotCmd = cmd
					return validFixturePayment(), nil
				},
			}, nil)
			w := httptest.NewRecorder()
			h.UpdatePayment(w, paymentRequest(t, http.MethodPatch, actor, tc.body),
				propertyID, paymentID)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
			}
			if present := gotCmd.EndDate != nil; present != tc.wantSet {
				t.Fatalf("command EndDate update present = %v, want %v", present, tc.wantSet)
			}
			if tc.wantSet {
				if tc.wantClr != (gotCmd.EndDate.Value == nil) {
					t.Fatalf("EndDate.Value = %v, cleared=%v", gotCmd.EndDate.Value, tc.wantClr)
				}
				if tc.wantDay != "" && (gotCmd.EndDate.Value == nil || gotCmd.EndDate.Value.Format(time.DateOnly) != tc.wantDay) {
					t.Fatalf("EndDate.Value = %v, want %s", gotCmd.EndDate.Value, tc.wantDay)
				}
			}
		})
	}
}

// TestUpdatePayment_RejectsLegacyPaymentFormField pins the strict decode
// after the migration shim of the removed «Форма оплаты» is gone
// (карта #1005, тикеты #1006–#1009): the field no longer exists in the
// contract, so an unknown-field body is rejected like any other.
func TestUpdatePayment_RejectsLegacyPaymentFormField(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())

	h := NewPaymentHandlers(&fakePaymentManager{}, nil)
	w := httptest.NewRecorder()
	h.UpdatePayment(w, paymentRequest(t, http.MethodPatch, actor,
		`{"title": "Новое название", "paymentForm": "cash"}`), propertyID, paymentID)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestCreatePayment_MapsReminderOffset(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())

	// Build a create payload with the given reminder lead time; the long
	// JSON stays assembled, not one long line.
	reminderCreateBody := func(days string) string {
		return `{"type":"expense","title":"Аренда","amountKopecks":5000000,` +
			`"recurrence":{"kind":"daily"},"categorySlug":"rent",` +
			`"reminderOffsetDays":` + days + `}`
	}

	cases := []struct {
		name     string
		body     string
		wantNil  bool
		wantDays int
	}{
		{"absent — no reminders", validCreateBody, true, 0},
		{"explicit 3", reminderCreateBody("3"), false, 3},
		{"explicit 7", reminderCreateBody("7"), false, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotCmd application.CreatePaymentCommand
			h := NewPaymentHandlers(&fakePaymentManager{
				create: func(_ context.Context, _, _ uuid.UUID, cmd application.CreatePaymentCommand) (domain.Payment, error) {
					gotCmd = cmd
					return validFixturePayment(), nil
				},
			}, nil)
			w := httptest.NewRecorder()
			h.CreatePayment(w, paymentRequest(t, http.MethodPost, actor, tc.body), propertyID)
			if w.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
			}
			if tc.wantNil {
				if gotCmd.ReminderOffsetDays != nil {
					t.Fatalf("command reminder offset = %v, want nil", gotCmd.ReminderOffsetDays)
				}
				return
			}
			if gotCmd.ReminderOffsetDays == nil || *gotCmd.ReminderOffsetDays != tc.wantDays {
				t.Fatalf("command reminder offset = %v, want %d", gotCmd.ReminderOffsetDays, tc.wantDays)
			}
		})
	}
}

func TestUpdatePayment_ReminderOffsetTriState(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())

	// Table fields mirror the endDate tri-state: wantSet — the command
	// carries a ReminderOffsetUpdate; wantClr — the update turns reminders
	// off; wantDays — the set lead time.
	cases := []struct {
		name     string
		body     string
		wantSet  bool
		wantClr  bool
		wantDays int
	}{
		{"omitted keeps", `{"title": "Новое название"}`, false, false, 0},
		{"null clears", `{"reminderOffsetDays": null}`, true, true, 0},
		{"1 sets", `{"reminderOffsetDays": 1}`, true, false, 1},
		{"7 sets", `{"reminderOffsetDays": 7}`, true, false, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotCmd application.UpdatePaymentCommand
			h := NewPaymentHandlers(&fakePaymentManager{
				update: func(_ context.Context, _, _, _ uuid.UUID, cmd application.UpdatePaymentCommand) (domain.Payment, error) {
					gotCmd = cmd
					return validFixturePayment(), nil
				},
			}, nil)
			w := httptest.NewRecorder()
			h.UpdatePayment(w, paymentRequest(t, http.MethodPatch, actor, tc.body),
				propertyID, paymentID)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
			}
			if present := gotCmd.ReminderOffsetDays != nil; present != tc.wantSet {
				t.Fatalf("command ReminderOffsetDays update present = %v, want %v", present, tc.wantSet)
			}
			if tc.wantSet {
				if tc.wantClr != (gotCmd.ReminderOffsetDays.Value == nil) {
					t.Fatalf("ReminderOffsetDays.Value = %v, cleared=%v", gotCmd.ReminderOffsetDays.Value, tc.wantClr)
				}
				if !tc.wantClr && (gotCmd.ReminderOffsetDays.Value == nil || *gotCmd.ReminderOffsetDays.Value != tc.wantDays) {
					t.Fatalf("ReminderOffsetDays.Value = %v, want %d", gotCmd.ReminderOffsetDays.Value, tc.wantDays)
				}
			}
		})
	}
}

func TestPaymentResponse_CarriesReminderOffset(t *testing.T) {
	t.Parallel()
	offset := 3
	respond := validFixturePayment()
	respond.ReminderOffsetDays = &offset

	h := NewPaymentHandlers(&fakePaymentManager{
		get: func(_ context.Context, _, _, _ uuid.UUID) (domain.Payment, error) {
			return respond, nil
		},
	}, nil)
	w := httptest.NewRecorder()
	h.GetPayment(w, paymentRequest(t, http.MethodGet, uuid.Must(uuid.NewV7()), ""),
		uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ReminderOffsetDays *int `json:"reminderOffsetDays"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ReminderOffsetDays == nil || *resp.ReminderOffsetDays != 3 {
		t.Fatalf("response reminderOffsetDays = %v, want 3", resp.ReminderOffsetDays)
	}
}

// TestGetPayment_CarriesRentalCompleted (#1158): the single-rule read passes
// the managing rental's «Завершена» state through next to the #818 gate
// flag — the payment screen gates «Изменить аренду» on it.
func TestGetPayment_CarriesRentalCompleted(t *testing.T) {
	t.Parallel()
	h := NewPaymentHandlers(&fakePaymentManager{
		get: func(_ context.Context, _, _, _ uuid.UUID) (domain.Payment, error) {
			return validFixturePayment(), nil
		},
		rentalManagedStatus: func(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
			return true, nil
		},
		rentalCompleted: func(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
			return true, nil
		},
	}, nil)
	w := httptest.NewRecorder()
	h.GetPayment(w, paymentRequest(t, http.MethodGet, uuid.Must(uuid.NewV7()), ""),
		uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		IsRentalManaged   bool `json:"isRentalManaged"`
		IsRentalCompleted bool `json:"isRentalCompleted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.IsRentalManaged || !resp.IsRentalCompleted {
		t.Fatalf("flags = managed:%v rentalCompleted:%v, want both true", resp.IsRentalManaged, resp.IsRentalCompleted)
	}
}

func TestDeletePayment_KeepOverdueDefault(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name     string
		params   openapi.DeletePaymentParams
		wantKeep bool
	}{
		{"default keeps the debt", openapi.DeletePaymentParams{}, true},
		{"explicit false deletes the debt", openapi.DeletePaymentParams{KeepOverdue: new(false)}, false},
		{"explicit true keeps the debt", openapi.DeletePaymentParams{KeepOverdue: new(true)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotKeep bool
			h := NewPaymentHandlers(&fakePaymentManager{
				del: func(_ context.Context, _, _, _ uuid.UUID, keepOverdue bool) error {
					gotKeep = keepOverdue
					return nil
				},
			}, nil)
			w := httptest.NewRecorder()
			h.DeletePayment(w, paymentRequest(t, http.MethodDelete, actor, ""),
				propertyID, paymentID, tc.params)
			if w.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204; body: %s", w.Code, w.Body.String())
			}
			if gotKeep != tc.wantKeep {
				t.Fatalf("keepOverdue = %v, want %v", gotKeep, tc.wantKeep)
			}
		})
	}
}

func TestPaymentHandlers_ErrorMapping(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantDetail string
	}{
		{"not found", application.ErrNotFound, http.StatusNotFound, testDetailNotFmt},
		{"forbidden", application.ErrForbidden, http.StatusForbidden, "Недостаточно прав для этого действия"},
		{"archived property", application.ErrArchivedProperty, http.StatusConflict, "Нельзя изменить архивный объект"},
		{"already paused", application.ErrAlreadyPaused, http.StatusConflict, "Платёж уже на паузе"},
		{"not paused", application.ErrNotPaused, http.StatusConflict, "Платёж не на паузе"},
		{"rental-managed", application.ErrRentManagedPayment, http.StatusConflict, "Платёж управляется арендой"},
		// The schedule window's specific detail answers before the shared
		// invalid-input table (ticket #1154).
		{
			"end date before first occurrence", application.ErrEndDateBeforeFirstOccurrence,
			http.StatusBadRequest, "Дата окончания не может быть раньше первого платежа",
		},
		{"invalid input", application.ErrInvalidInput, http.StatusBadRequest, "Некорректные данные платежа"},
		{"wrapped not found", wrapped(application.ErrNotFound), http.StatusNotFound, testDetailNotFmt},
		{"internal", errors.New("boom"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewPaymentHandlers(&fakePaymentManager{
				get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Payment, error) {
					return domain.Payment{}, tc.err
				},
				pause: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Payment, error) {
					return domain.Payment{}, tc.err
				},
			}, nil)
			w := httptest.NewRecorder()
			h.GetPayment(w, paymentRequest(t, http.MethodGet, actor, ""),
				propertyID, paymentID)
			if w.Code != tc.wantStatus {
				t.Fatalf("get: status = %d, want %d; body: %s", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantDetail != "" && !strings.Contains(w.Body.String(), tc.wantDetail) {
				t.Fatalf("get: body %s lacks detail %q", w.Body.String(), tc.wantDetail)
			}

			w2 := httptest.NewRecorder()
			h.PausePayment(w2, paymentRequest(t, http.MethodPost, actor, ""),
				propertyID, paymentID)
			if w2.Code != tc.wantStatus {
				t.Fatalf("pause: status = %d, want %d; body: %s", w2.Code, tc.wantStatus, w2.Body.String())
			}
		})
	}
}

func TestListPayments_MapsItems(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	h := NewPaymentHandlers(&fakePaymentManager{
		list: func(context.Context, uuid.UUID, uuid.UUID, string) ([]application.PaymentListItem, error) {
			return nil, nil
		},
	}, nil)

	w := httptest.NewRecorder()
	h.ListPayments(w, paymentRequest(t, http.MethodGet, actor, ""),
		propertyID, openapi.ListPaymentsParams{})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// An empty property answers with an empty items array, never null.
	if resp.Items == nil {
		t.Fatalf("items = null, want []")
	}
}

// validFixturePayment is the minimal response-mappable rule: the mapping
// requires a constructed recurrence, so zero payments would 500.
func validFixturePayment() domain.Payment {
	return domain.Payment{
		Type:          domain.TypeExpense,
		Title:         "t",
		AmountKopecks: 100,
		Recurrence:    domain.NewDailyRecurrence(),
	}
}

// mustWeekly builds a sorted weekly recurrence for fixtures.
func mustWeekly(t *testing.T, weekdays ...int) domain.Recurrence {
	t.Helper()
	days := make([]time.Weekday, len(weekdays))
	for i, wd := range weekdays {
		days[i] = time.Weekday(wd)
	}
	rec, err := domain.NewWeeklyRecurrence(days)
	if err != nil {
		t.Fatalf("fixture recurrence: %v", err)
	}
	return rec
}

// wrapped wraps a sentinel the way the application layer wraps store errors.
func wrapped(err error) error {
	return fmt.Errorf("get payment: %w", err)
}

// TestListPaymentChanges pins the changes endpoint's wire shape (ADR 0065):
// the typed diff values travel verbatim, the cursors surface as optionals,
// and the contract errors map onto 400/404.
func TestListPaymentChanges(t *testing.T) {
	t.Parallel()
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	actorID := uuid.Must(uuid.NewV7())
	entryID := uuid.Must(uuid.NewV7())
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	h := NewPaymentHandlers(changesFake(t, paymentID, entryID, actorID, at), nil)
	w := httptest.NewRecorder()
	limit := 7
	h.ListPaymentChanges(w, httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actorID), http.MethodGet, "/", nil,
	), propertyID, paymentID, openapi.ListPaymentChangesParams{Limit: &limit})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want 200", w.Code, w.Body)
	}

	var body struct {
		Items      []changeItemWire `json:"items"`
		NextCursor *string          `json:"next_cursor"`
		PrevCursor *string          `json:"prev_cursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
	assertChangeItem(t, body.Items[0], entryID, actorID, at)
	assertAmountChange(t, body.Items[0].Changes[0])
	if body.NextCursor == nil || *body.NextCursor != "next-blob" ||
		body.PrevCursor == nil || *body.PrevCursor != "prev-blob" {
		t.Fatalf("cursors = %s", w.Body)
	}
}

// TestListPaymentChanges_ErrorMapping pins the contract statuses: the
// privacy 404 and the malformed-cursor 400.
func TestListPaymentChanges_ErrorMapping(t *testing.T) {
	t.Parallel()
	propertyID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	actorID := uuid.Must(uuid.NewV7())

	notFound := &fakePaymentManager{}
	notFound.paymentChanges = func(
		_ context.Context, _, _, _ uuid.UUID, _ application.ChangesQuery,
	) (application.ChangeLogPage, error) {
		return application.ChangeLogPage{}, application.ErrNotFound
	}
	badCursor := &fakePaymentManager{}
	badCursor.paymentChanges = func(
		_ context.Context, _, _, _ uuid.UUID, _ application.ChangesQuery,
	) (application.ChangeLogPage, error) {
		return application.ChangeLogPage{}, fmt.Errorf("payment change cursor: %w", application.ErrInvalidInput)
	}

	cases := []struct {
		name  string
		fake  *fakePaymentManager
		wants int
	}{
		{"privacy 404", notFound, http.StatusNotFound},
		{"cursor 400", badCursor, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			NewPaymentHandlers(tc.fake, nil).ListPaymentChanges(w, httptest.NewRequestWithContext(
				httpsupport.WithUserID(t.Context(), actorID), http.MethodGet, "/", nil,
			), propertyID, paymentID, openapi.ListPaymentChangesParams{})
			if w.Code != tc.wants {
				t.Fatalf("status = %d, body = %s, want %d", w.Code, w.Body, tc.wants)
			}
		})
	}
}

// changesFake is the one-page fixture fake: the seeded updated entry with an
// amount and an end-date diff, both cursors set.
func changesFake(
	t *testing.T, paymentID, entryID, actorID uuid.UUID, at time.Time,
) *fakePaymentManager {
	t.Helper()
	f := &fakePaymentManager{}
	f.paymentChanges = func(
		_ context.Context, _, _, gotPayment uuid.UUID, q application.ChangesQuery,
	) (application.ChangeLogPage, error) {
		if gotPayment != paymentID {
			t.Fatalf("payment id = %s, want %s", gotPayment, paymentID)
		}
		if q.Limit != 7 {
			t.Fatalf("limit = %d, want the wire's 7", q.Limit)
		}
		return application.ChangeLogPage{
			Items: []domain.ChangeEntry{{
				ID:      entryID,
				ActorID: actorID,
				Action:  domain.ChangeUpdated,
				Changes: []domain.FieldChange{
					{Field: domain.ChangeFieldAmount, Old: json.RawMessage("2500000"), New: json.RawMessage("5000000")},
					{Field: domain.ChangeFieldEndDate, Old: json.RawMessage("null"), New: json.RawMessage(`"2027-01-31"`)},
				},
				CreatedAt: at,
			}},
			NextCursor: "next-blob",
			PrevCursor: "prev-blob",
		}, nil
	}
	return f
}

// changeItemWire is the decoded wire shape of one PaymentChangeItem.
type changeItemWire struct {
	ID        string            `json:"id"`
	ActorID   string            `json:"actor_id"`
	Action    string            `json:"action"`
	CreatedAt time.Time         `json:"created_at"`
	Changes   []json.RawMessage `json:"changes"`
}

// assertChangeItem checks the entry's snapshot fields on the wire.
func assertChangeItem(t *testing.T, item changeItemWire, entryID, actorID uuid.UUID, at time.Time) {
	t.Helper()
	if item.ID != entryID.String() || item.ActorID != actorID.String() ||
		item.Action != "updated" || !item.CreatedAt.Equal(at) {
		t.Fatalf("item = %+v", item)
	}
	if len(item.Changes) != 2 {
		t.Fatalf("changes = %s, want 2 fields", item.Changes)
	}
}

// assertAmountChange checks the typed values travel verbatim: the kopecks as
// JSON numbers, never strings-with-formatting.
func assertAmountChange(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var change struct {
		Field string          `json:"field"`
		Old   json.RawMessage `json:"old"`
		New   json.RawMessage `json:"new"`
	}
	if err := json.Unmarshal(raw, &change); err != nil {
		t.Fatalf("decode change: %v", err)
	}
	if change.Field != "amount_kopecks" ||
		string(change.Old) != "2500000" || string(change.New) != "5000000" {
		t.Fatalf("amount change = %s — the typed values must travel verbatim", raw)
	}
}
