package http

// The contract tests of the second payments contracts slice (ticket #461):
// the operations listings' parameter folding, the pay-now error mapping and
// the favorite PUT's body handling — against func-backed doubles, mirroring
// payment_handlers_test.go.

import (
	"context"
	"encoding/json"
	"errors"
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
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// fakeOperationsManager is the func-backed OperationsManager double: every
// use case is optional; an unset one fails the test loudly instead of
// silently succeeding (ADR 0035 test doubles). The func fields back the
// port's methods one to one; the field order is alphabetical, unlike the
// port.
type fakeOperationsManager struct {
	create func(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.CreateOperationCommand,
	) (application.OperationListItem, error)
	byPay func(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
	byProp func(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
	del        func(ctx context.Context, actor, propertyID, operationID uuid.UUID) error
	get        func(ctx context.Context, actor, propertyID, operationID uuid.UUID) (application.OperationListItem, error)
	globalList func(
		ctx context.Context, actor uuid.UUID, cmd application.GlobalOperationsListQuery,
	) (application.GlobalOperationsPage, error)
	globalSummarize func(
		ctx context.Context, actor uuid.UUID, cmd application.GlobalOperationsSummaryQuery,
	) (application.OperationsSummary, error)
	pay       func(ctx context.Context, actor, propertyID, operationID uuid.UUID) (application.OperationListItem, error)
	summarize func(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.OperationsSummaryQuery,
	) (application.OperationsSummary, error)
}

func (f *fakeOperationsManager) CreateOperation(
	ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreateOperationCommand,
) (application.OperationListItem, error) {
	if f.create == nil {
		return application.OperationListItem{}, errors.New("unexpected CreateOperation call")
	}
	return f.create(ctx, actor, propertyID, cmd)
}

func (f *fakeOperationsManager) DeleteOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) error {
	if f.del == nil {
		return errors.New("unexpected DeleteOperation call")
	}
	return f.del(ctx, actor, propertyID, operationID)
}

func (f *fakeOperationsManager) GetOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) (application.OperationListItem, error) {
	if f.get == nil {
		return application.OperationListItem{}, errors.New("unexpected GetOperation call")
	}
	return f.get(ctx, actor, propertyID, operationID)
}

func (f *fakeOperationsManager) PayOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) (application.OperationListItem, error) {
	if f.pay == nil {
		return application.OperationListItem{}, errors.New("unexpected PayOperation call")
	}
	return f.pay(ctx, actor, propertyID, operationID)
}

func (f *fakeOperationsManager) ListPaymentOperations(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
	cmd application.OperationsListQuery,
) ([]application.OperationListItem, error) {
	if f.byPay == nil {
		return nil, errors.New("unexpected ListPaymentOperations call")
	}
	return f.byPay(ctx, actor, propertyID, paymentID, cmd)
}

func (f *fakeOperationsManager) ListPropertyOperations(
	ctx context.Context, actor, propertyID uuid.UUID, cmd application.OperationsListQuery,
) ([]application.OperationListItem, error) {
	if f.byProp == nil {
		return nil, errors.New("unexpected ListPropertyOperations call")
	}
	return f.byProp(ctx, actor, propertyID, cmd)
}

func (f *fakeOperationsManager) SummarizePropertyOperations(
	ctx context.Context, actor, propertyID uuid.UUID, cmd application.OperationsSummaryQuery,
) (application.OperationsSummary, error) {
	if f.summarize == nil {
		return application.OperationsSummary{}, errors.New("unexpected SummarizePropertyOperations call")
	}
	return f.summarize(ctx, actor, propertyID, cmd)
}

func (f *fakeOperationsManager) ListGlobalOperations(
	ctx context.Context, actor uuid.UUID, cmd application.GlobalOperationsListQuery,
) (application.GlobalOperationsPage, error) {
	if f.globalList == nil {
		return application.GlobalOperationsPage{}, errors.New("unexpected ListGlobalOperations call")
	}
	return f.globalList(ctx, actor, cmd)
}

func (f *fakeOperationsManager) SummarizeGlobalOperations(
	ctx context.Context, actor uuid.UUID, cmd application.GlobalOperationsSummaryQuery,
) (application.OperationsSummary, error) {
	if f.globalSummarize == nil {
		return application.OperationsSummary{}, errors.New("unexpected SummarizeGlobalOperations call")
	}
	return f.globalSummarize(ctx, actor, cmd)
}

// fixtureOperation is the tests' fixture operation; a paid status carries a
// paid_date so the response mapping stays total.
func fixtureOperation(status domain.OperationStatus) domain.Operation {
	id := uuid.Must(uuid.NewV7())
	propID := uuid.Must(uuid.NewV7())
	paymentID := uuid.Must(uuid.NewV7())
	date := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	op := domain.Operation{
		ID: id, PropertyID: propID, PaymentID: &paymentID,
		Origin: domain.OriginPayment, Date: date, Status: status,
		Type: domain.TypeExpense, Title: "ЖКУ", AmountKopecks: 500000,
		CategoryLabel: "Коммунальные услуги",
	}
	form := domain.FormTransfer
	op.PaymentForm = &form
	slug := "utilities"
	op.CategorySlug = &slug
	if status == domain.StatusPaid {
		paid := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
		op.PaidDate = &paid
	}
	return op
}

func TestOperationsHandlers_RequireAuth(t *testing.T) {
	t.Parallel()
	h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
	propertyID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name string
		call func(w http.ResponseWriter, r *http.Request)
	}{
		{"create", func(w http.ResponseWriter, r *http.Request) {
			h.CreateOperation(w, r, propertyID)
		}},
		{"get", func(w http.ResponseWriter, r *http.Request) {
			h.GetOperation(w, r, propertyID, otherID)
		}},
		{"delete", func(w http.ResponseWriter, r *http.Request) {
			h.DeleteOperation(w, r, propertyID, otherID)
		}},
		{"list by payment", func(w http.ResponseWriter, r *http.Request) {
			h.ListPaymentOperations(w, r, propertyID, otherID, openapi.ListPaymentOperationsParams{})
		}},
		{"list by property", func(w http.ResponseWriter, r *http.Request) {
			h.ListPropertyOperations(w, r, propertyID, openapi.ListPropertyOperationsParams{})
		}},
		{"pay", func(w http.ResponseWriter, r *http.Request) {
			h.PayOperation(w, r, propertyID, otherID)
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

func TestListPaymentOperations_FoldsParamsIntoCommand(t *testing.T) {
	t.Parallel()

	var gotCmd application.OperationsListQuery
	svc := &fakeOperationsManager{
		byPay: func(
			_ context.Context, _, _, _ uuid.UUID, cmd application.OperationsListQuery,
		) ([]application.OperationListItem, error) {
			gotCmd = cmd
			return []application.OperationListItem{
				{Operation: fixtureOperation(domain.StatusPlanned), ViewStatus: domain.ViewStatusOverdue},
			}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	status := openapi.ListPaymentOperationsParamsStatusOverdue
	from := openapi_types.Date{Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	to := openapi_types.Date{Time: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}
	asc := openapi.ListPaymentOperationsParamsOrderAsc
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListPaymentOperations(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()),
		openapi.ListPaymentOperationsParams{
			Status:   &status,
			DateFrom: &from,
			DateTo:   &to,
			Order:    &asc,
			Limit:    intPtr(10),
			Offset:   intPtr(20),
		})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotCmd.Status == nil || *gotCmd.Status != domain.ViewStatusOverdue {
		t.Errorf("cmd.Status = %v, want overdue", gotCmd.Status)
	}
	if gotCmd.Asc != true {
		t.Errorf("order=asc lost: Asc = %v, want true", gotCmd.Asc)
	}
	if gotCmd.DateFrom == nil || gotCmd.DateTo == nil {
		t.Errorf("period = %v..%v, want both bounds carried", gotCmd.DateFrom, gotCmd.DateTo)
	}
	if gotCmd.Limit != 10 || gotCmd.Offset != 20 {
		t.Errorf("limit/offset = %d/%d, want 10/20", gotCmd.Limit, gotCmd.Offset)
	}

	var body struct {
		Items []openapi.OperationResponse `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
	if body.Items[0].Status != openapi.OperationResponseStatusOverdue {
		t.Errorf("wire status = %q, want the computed overdue view status", body.Items[0].Status)
	}
}

func TestListPaymentOperations_DefaultsAreDescPage50(t *testing.T) {
	t.Parallel()

	var gotCmd application.OperationsListQuery
	svc := &fakeOperationsManager{
		byPay: func(
			_ context.Context, _, _, _ uuid.UUID, cmd application.OperationsListQuery,
		) ([]application.OperationListItem, error) {
			gotCmd = cmd
			return []application.OperationListItem{}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListPaymentOperations(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()),
		openapi.ListPaymentOperationsParams{})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotCmd.Asc {
		t.Error("default direction = asc, want the descending contract default")
	}
	if gotCmd.Status != nil {
		t.Errorf("status = %v, want none", gotCmd.Status)
	}
	if err := application.PrepareOperationsQuery(&gotCmd); err != nil {
		t.Fatalf("prepare default command: %v", err)
	}
	if gotCmd.Limit != application.DefaultOperationsPageSize {
		t.Errorf("default limit = %d, want %d", gotCmd.Limit, application.DefaultOperationsPageSize)
	}
}

func TestListPropertyOperations_ForwardsToPort(t *testing.T) {
	t.Parallel()

	planned := openapi.ListPropertyOperationsParamsStatusPlanned
	var gotActor, gotProp uuid.UUID
	var gotCmd application.OperationsListQuery
	svc := &fakeOperationsManager{
		byProp: func(
			_ context.Context, actor, prop uuid.UUID, cmd application.OperationsListQuery,
		) ([]application.OperationListItem, error) {
			gotActor, gotProp, gotCmd = actor, prop, cmd
			return []application.OperationListItem{
				{Operation: fixtureOperation(domain.StatusPlanned), ViewStatus: domain.ViewStatusPlanned},
			}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())
	propID := uuid.Must(uuid.NewV7())

	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListPropertyOperations(w, req, propID, openapi.ListPropertyOperationsParams{
		Status: &planned,
		Limit:  intPtr(3),
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotActor != actor || gotProp != propID {
		t.Errorf("actor/property = %s/%s, want %s/%s", gotActor, gotProp, actor, propID)
	}
	if gotCmd.Status == nil || *gotCmd.Status != domain.ViewStatusPlanned || gotCmd.Limit != 3 {
		t.Errorf("cmd = %+v, want planned with limit 3", gotCmd)
	}
	var items struct {
		Items []openapi.OperationResponse `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items.Items) != 1 || items.Items[0].Status != openapi.OperationResponseStatusPlanned {
		t.Errorf("wire = %+v, want one planned item", items.Items)
	}
}

func TestOperationsLists_RejectUnknownFilterValues(t *testing.T) {
	t.Parallel()
	h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
	actor := uuid.Must(uuid.NewV7())
	bogus := openapi.ListPaymentOperationsParamsStatus("archived")

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	h.ListPaymentOperations(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()),
		openapi.ListPaymentOperationsParams{Status: &bogus})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an out-of-vocabulary filter", w.Code)
	}
}

func TestListPropertyOperations_MapsApplicationErrors(t *testing.T) {
	t.Parallel()

	svc := &fakeOperationsManager{
		byProp: func(context.Context, uuid.UUID, uuid.UUID, application.OperationsListQuery) ([]application.OperationListItem, error) {
			return nil, application.ErrNotFound
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListPropertyOperations(w, req, uuid.Must(uuid.NewV7()), openapi.ListPropertyOperationsParams{})

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want the privacy 404", w.Code)
	}
}

func TestDeleteOperation_CancelsAndMapsErrors(t *testing.T) {
	t.Parallel()

	t.Run("cancels with 204", func(t *testing.T) {
		t.Parallel()
		var gotActor, gotProperty, gotOperation uuid.UUID
		svc := &fakeOperationsManager{
			del: func(_ context.Context, actor, propertyID, operationID uuid.UUID) error {
				gotActor, gotProperty, gotOperation = actor, propertyID, operationID
				return nil
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodDelete, "/operation", nil,
		)
		w := httptest.NewRecorder()
		propertyID := uuid.Must(uuid.NewV7())
		operationID := uuid.Must(uuid.NewV7())
		h.DeleteOperation(w, req, propertyID, operationID)

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", w.Code, w.Body.String())
		}
		if gotActor != actor || gotProperty != propertyID || gotOperation != operationID {
			t.Errorf("actor/property/operation = %s/%s/%s, want %s/%s/%s",
				gotActor, gotProperty, gotOperation, actor, propertyID, operationID)
		}
	})

	t.Run("a foreign or cancelled operation is the privacy 404", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			del: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
				return application.ErrNotFound
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodDelete, "/operation", nil,
		)
		w := httptest.NewRecorder()
		h.DeleteOperation(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want the privacy 404", w.Code)
		}
	})

	t.Run("a viewer deletes nothing", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			del: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
				return application.ErrForbidden
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodDelete, "/operation", nil,
		)
		w := httptest.NewRecorder()
		h.DeleteOperation(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})

	t.Run("an archived property is the 409", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			del: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
				return application.ErrArchivedProperty
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodDelete, "/operation", nil,
		)
		w := httptest.NewRecorder()
		h.DeleteOperation(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", w.Code)
		}
	})
}

func TestGetOperation_ReturnsOperationAndMapsErrors(t *testing.T) {
	t.Parallel()

	paidOp := fixtureOperation(domain.StatusPaid)

	t.Run("returns the operation with the computed view", func(t *testing.T) {
		t.Parallel()
		var gotActor, gotProperty, gotOperation uuid.UUID
		svc := &fakeOperationsManager{
			get: func(_ context.Context, actor, propertyID, operationID uuid.UUID) (application.OperationListItem, error) {
				gotActor, gotProperty, gotOperation = actor, propertyID, operationID
				return application.OperationListItem{
					Operation:  paidOp,
					ViewStatus: domain.ViewStatusPaid,
				}, nil
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operation", nil,
		)
		w := httptest.NewRecorder()
		propertyID := uuid.Must(uuid.NewV7())
		h.GetOperation(w, req, propertyID, paidOp.ID)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		if gotActor != actor || gotProperty != propertyID || gotOperation != paidOp.ID {
			t.Errorf("actor/property/operation = %s/%s/%s, want %s/%s/%s",
				gotActor, gotProperty, gotOperation, actor, propertyID, paidOp.ID)
		}
		var body openapi.OperationResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Id != paidOp.ID || body.Status != openapi.OperationResponseStatusPaid {
			t.Errorf("wire = %s/%s, want the requested id with the computed paid status",
				body.Id, body.Status)
		}
	})

	t.Run("a foreign operation is the privacy 404", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			get: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (application.OperationListItem, error) {
				return application.OperationListItem{}, application.ErrNotFound
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operation", nil,
		)
		w := httptest.NewRecorder()
		h.GetOperation(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want the privacy 404", w.Code)
		}
	})
}

func TestPayOperation_HappyPathAndConflicts(t *testing.T) {
	t.Parallel()

	paidOp := fixtureOperation(domain.StatusPaid)

	t.Run("returns the paid operation", func(t *testing.T) {
		t.Parallel()
		var gotID uuid.UUID
		svc := &fakeOperationsManager{
			pay: func(_ context.Context, _, _, operationID uuid.UUID) (application.OperationListItem, error) {
				gotID = operationID
				return application.OperationListItem{
					Operation:  paidOp,
					ViewStatus: domain.ViewStatusPaid,
				}, nil
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodPost, "/pay", nil,
		)
		w := httptest.NewRecorder()
		operationID := uuid.Must(uuid.NewV7())
		h.PayOperation(w, req, paidOp.PropertyID, operationID)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		if gotID != operationID {
			t.Errorf("paid operation id = %s, want %s", gotID, operationID)
		}
		var body openapi.OperationResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Status != openapi.OperationResponseStatusPaid {
			t.Errorf("status = %q, want paid", body.Status)
		}
		if body.PaidDate == nil || !body.PaidDate.Equal(paidOp.PaidDate.UTC()) {
			t.Errorf("paidDate = %v, want %v", body.PaidDate, paidOp.PaidDate)
		}
	})

	t.Run("a repeated pay maps to 409", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			pay: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (application.OperationListItem, error) {
				return application.OperationListItem{}, application.ErrAlreadyPaid
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodPost, "/pay", nil,
		)
		w := httptest.NewRecorder()
		h.PayOperation(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 on a repeated pay", w.Code)
		}
	})

	t.Run("a viewer pays nothing", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			pay: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (application.OperationListItem, error) {
				return application.OperationListItem{}, application.ErrForbidden
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodPost, "/pay", nil,
		)
		w := httptest.NewRecorder()
		h.PayOperation(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
	})
}

func TestSetPaymentFavorite_BodyAndResponse(t *testing.T) {
	t.Parallel()

	t.Run("folds the body into the port call", func(t *testing.T) {
		t.Parallel()

		rule := validFixturePayment()
		var gotFlag bool
		svc := &fakePaymentManager{
			favorite: func(
				_ context.Context, _, _, _ uuid.UUID, favorite bool,
			) (domain.Payment, error) {
				gotFlag = favorite
				rule.IsFavorite = favorite
				return rule, nil
			},
		}
		h := NewPaymentHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		body := paymentRequest(t, http.MethodPut, actor, `{"favorite": false}`)
		w := httptest.NewRecorder()
		h.SetPaymentFavorite(w, body, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		if gotFlag {
			t.Error("favorite flag = true, want the delivered false")
		}
		var wire openapi.PaymentResponse
		if err := json.Unmarshal(w.Body.Bytes(), &wire); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if wire.IsFavorite {
			t.Error("isFavorite = true, want false")
		}
	})

	t.Run("malformed bodies are 400", func(t *testing.T) {
		t.Parallel()
		h := NewPaymentHandlers(&fakePaymentManager{}, nil)
		actor := uuid.Must(uuid.NewV7())

		cases := map[string]string{
			"broken json":  `{"favorite":`,
			"missing flag": `{}`,
			"wrong type":   `{"favorite": "yes"}`,
			"unknown":      `{"starred": true}`,
		}
		for name, raw := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				body := paymentRequest(t, http.MethodPut, actor, raw)
				w := httptest.NewRecorder()
				h.SetPaymentFavorite(w, body, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400 (%s)", w.Code, raw)
				}
			})
		}
	})
}

func intPtr(v int) *int { return new(v) }

func TestSummarizePropertyOperations_FoldsParamsIntoCommand(t *testing.T) {
	t.Parallel()

	var gotActor, gotProp uuid.UUID
	var gotCmd application.OperationsSummaryQuery
	svc := &fakeOperationsManager{
		summarize: func(
			_ context.Context, actor, prop uuid.UUID, cmd application.OperationsSummaryQuery,
		) (application.OperationsSummary, error) {
			gotActor, gotProp, gotCmd = actor, prop, cmd
			return application.OperationsSummary{}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())
	propID := uuid.Must(uuid.NewV7())

	paid := openapi.SummarizePropertyOperationsParamsStatusPaid
	expense := openapi.SummarizePropertyOperationsParamsTypeExpense
	from := openapi_types.Date{Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	to := openapi_types.Date{Time: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations/summary", nil,
	)
	w := httptest.NewRecorder()
	h.SummarizePropertyOperations(w, req, propID, openapi.SummarizePropertyOperationsParams{
		Status:   &paid,
		Type:     &expense,
		DateFrom: &from,
		DateTo:   &to,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotActor != actor || gotProp != propID {
		t.Errorf("actor/property = %s/%s, want %s/%s", gotActor, gotProp, actor, propID)
	}
	if gotCmd.Status == nil || *gotCmd.Status != domain.ViewStatusPaid {
		t.Errorf("cmd.Status = %v, want paid", gotCmd.Status)
	}
	if gotCmd.Type == nil || *gotCmd.Type != domain.TypeExpense {
		t.Errorf("cmd.Type = %v, want expense", gotCmd.Type)
	}
	if gotCmd.DateFrom == nil || gotCmd.DateTo == nil {
		t.Errorf("period = %v..%v, want both bounds carried", gotCmd.DateFrom, gotCmd.DateTo)
	}
}

func TestSummarizePropertyOperations_ResponseShape(t *testing.T) {
	t.Parallel()

	svc := &fakeOperationsManager{
		summarize: func(
			context.Context, uuid.UUID, uuid.UUID, application.OperationsSummaryQuery,
		) (application.OperationsSummary, error) {
			return application.OperationsSummary{
				IncomeTotalKopecks:  6650000,
				ExpenseTotalKopecks: 1700000,
				Categories: []application.CategorySummary{
					{Slug: testSlugRent, Label: testLabelRent, Type: domain.TypeIncome, TotalKopecks: 5650000},
				},
			}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)

	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), uuid.Must(uuid.NewV7())), http.MethodGet, "/operations/summary", nil,
	)
	w := httptest.NewRecorder()
	h.SummarizePropertyOperations(w, req, uuid.Must(uuid.NewV7()), openapi.SummarizePropertyOperationsParams{})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body openapi.OperationsSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.IncomeTotalKopecks != 6650000 || body.ExpenseTotalKopecks != 1700000 {
		t.Errorf("totals = %d/%d, want 6 650 000/1 700 000",
			body.IncomeTotalKopecks, body.ExpenseTotalKopecks)
	}
	if len(body.Categories) != 1 ||
		body.Categories[0].CategorySlug != testSlugRent ||
		body.Categories[0].CategoryLabel != testLabelRent ||
		body.Categories[0].Type != openapi.OperationsSummaryCategoryTypeIncome {
		t.Errorf("categories = %+v, want the single rent row", body.Categories)
	}
}

func TestSummarizePropertyOperations_MapsErrorsAndAuth(t *testing.T) {
	t.Parallel()

	t.Run("requires auth", func(t *testing.T) {
		t.Parallel()
		h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
		w := httptest.NewRecorder()
		h.SummarizePropertyOperations(w,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
			uuid.Must(uuid.NewV7()), openapi.SummarizePropertyOperationsParams{})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("rejects an out-of-vocabulary type", func(t *testing.T) {
		t.Parallel()
		h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
		actor := uuid.Must(uuid.NewV7())
		bogus := openapi.SummarizePropertyOperationsParamsType("both")

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/", nil,
		)
		w := httptest.NewRecorder()
		h.SummarizePropertyOperations(w, req, uuid.Must(uuid.NewV7()),
			openapi.SummarizePropertyOperationsParams{Type: &bogus})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for an out-of-vocabulary type", w.Code)
		}
	})

	t.Run("a foreign property is the privacy 404", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			summarize: func(context.Context, uuid.UUID, uuid.UUID, application.OperationsSummaryQuery) (application.OperationsSummary, error) {
				return application.OperationsSummary{}, application.ErrNotFound
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/", nil,
		)
		w := httptest.NewRecorder()
		h.SummarizePropertyOperations(w, req, uuid.Must(uuid.NewV7()), openapi.SummarizePropertyOperationsParams{})

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want the privacy 404", w.Code)
		}
	})
}

func TestListPropertyOperations_FoldsTypeAndCategoryFilters(t *testing.T) {
	t.Parallel()

	var gotCmd application.OperationsListQuery
	svc := &fakeOperationsManager{
		byProp: func(
			_ context.Context, _, _ uuid.UUID, cmd application.OperationsListQuery,
		) ([]application.OperationListItem, error) {
			gotCmd = cmd
			return []application.OperationListItem{}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	expense := openapi.ListPropertyOperationsParamsTypeExpense
	categories := "rent, utilities ,,security"
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListPropertyOperations(w, req, uuid.Must(uuid.NewV7()), openapi.ListPropertyOperationsParams{
		Type:     &expense,
		Category: &categories,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotCmd.Type == nil || *gotCmd.Type != domain.TypeExpense {
		t.Errorf("cmd.Type = %v, want expense", gotCmd.Type)
	}
	want := []string{testSlugRent, "utilities", "security"}
	if len(gotCmd.Categories) != len(want) {
		t.Fatalf("cmd.Categories = %v, want %v", gotCmd.Categories, want)
	}
	for i, slug := range want {
		if gotCmd.Categories[i] != slug {
			t.Errorf("cmd.Categories[%d] = %q, want %q", i, gotCmd.Categories[i], slug)
		}
	}

	t.Run("an empty category value disables the filter", func(t *testing.T) {
		t.Parallel()
		var gotCmd application.OperationsListQuery
		svc := &fakeOperationsManager{
			byProp: func(
				_ context.Context, _, _ uuid.UUID, cmd application.OperationsListQuery,
			) ([]application.OperationListItem, error) {
				gotCmd = cmd
				return []application.OperationListItem{}, nil
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())
		empty := " , "

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
		)
		w := httptest.NewRecorder()
		h.ListPropertyOperations(w, req, uuid.Must(uuid.NewV7()),
			openapi.ListPropertyOperationsParams{Category: &empty})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if len(gotCmd.Categories) != 0 {
			t.Errorf("cmd.Categories = %v, want no filter", gotCmd.Categories)
		}
	})
}

// globalFeedCall captures the folded command of a global listing call and
// answers with one paid row labeled by the given property name — the folding
// tests' shared double.
func globalFeedCall(gotCmd *application.GlobalOperationsListQuery, propertyName string) *fakeOperationsManager {
	return &fakeOperationsManager{
		globalList: func(
			_ context.Context, _ uuid.UUID, cmd application.GlobalOperationsListQuery,
		) (application.GlobalOperationsPage, error) {
			*gotCmd = cmd
			item := application.OperationListItem{
				Operation:  fixtureOperation(domain.StatusPaid),
				ViewStatus: domain.ViewStatusPaid,
			}
			item.PropertyName = propertyName
			return application.GlobalOperationsPage{Items: []application.OperationListItem{item}}, nil
		},
	}
}

func TestListOperations_FoldsParamsIntoCommand(t *testing.T) {
	t.Parallel()

	propA := uuid.Must(uuid.NewV7())
	propB := uuid.Must(uuid.NewV7())
	var gotCmd application.GlobalOperationsListQuery
	h := NewOperationsHandlers(globalFeedCall(&gotCmd, "Моя квартира"), nil)
	actor := uuid.Must(uuid.NewV7())

	propertyIds := propA.String() + ", " + propB.String() + " ,"
	expense := openapi.ListOperationsParamsTypeExpense
	categories := "rent, utilities"
	asc := openapi.ListOperationsParamsOrderAsc
	from := openapi_types.Date{Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	to := openapi_types.Date{Time: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}
	cursor := testEchoedCursor
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListOperations(w, req, openapi.ListOperationsParams{
		PropertyIds: &propertyIds,
		Type:        &expense,
		Category:    &categories,
		Order:       &asc,
		DateFrom:    &from,
		DateTo:      &to,
		Limit:       intPtr(10),
		Cursor:      &cursor,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if len(gotCmd.PropertyIDs) != 2 || gotCmd.PropertyIDs[0] != propA || gotCmd.PropertyIDs[1] != propB {
		t.Errorf("cmd.PropertyIDs = %v, want [%s %s] — whitespace around ids ignored", gotCmd.PropertyIDs, propA, propB)
	}
	if gotCmd.Type == nil || *gotCmd.Type != domain.TypeExpense {
		t.Errorf("cmd.Type = %v, want expense", gotCmd.Type)
	}
	if len(gotCmd.Categories) != 2 || gotCmd.Categories[0] != testSlugRent {
		t.Errorf("cmd.Categories = %v, want [%s utilities]", gotCmd.Categories, testSlugRent)
	}
	if !gotCmd.Asc || gotCmd.Limit != 10 || gotCmd.Cursor != testEchoedCursor {
		t.Errorf("cmd pagination/order = %+v, want asc 10 with the echoed cursor", gotCmd)
	}
	if gotCmd.DateFrom == nil || gotCmd.DateTo == nil {
		t.Errorf("period = %v..%v, want both bounds carried", gotCmd.DateFrom, gotCmd.DateTo)
	}
}

func TestListOperations_ResponseCarriesRowLabelAndPaidView(t *testing.T) {
	t.Parallel()

	var gotCmd application.GlobalOperationsListQuery
	h := NewOperationsHandlers(globalFeedCall(&gotCmd, "Моя квартира"), nil)

	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), uuid.Must(uuid.NewV7())), http.MethodGet, "/operations", nil,
	)
	w := httptest.NewRecorder()
	h.ListOperations(w, req, openapi.ListOperationsParams{})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []openapi.OperationResponse `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(body.Items))
	}
	if body.Items[0].PropertyName == nil || *body.Items[0].PropertyName != "Моя квартира" {
		t.Errorf("wire propertyName = %v, want the global row label", body.Items[0].PropertyName)
	}
	if body.Items[0].Status != openapi.OperationResponseStatusPaid {
		t.Errorf("wire status = %q, want paid — the feed's only view status", body.Items[0].Status)
	}
}

func TestListOperations_RejectsBadPropertyIdsAndMapsPrivacy(t *testing.T) {
	t.Parallel()

	t.Run("a non-uuid entry is 400", func(t *testing.T) {
		t.Parallel()
		h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
		actor := uuid.Must(uuid.NewV7())
		bogus := "not-a-uuid"

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
		)
		w := httptest.NewRecorder()
		h.ListOperations(w, req, openapi.ListOperationsParams{PropertyIds: &bogus})

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for a non-uuid propertyIds entry", w.Code)
		}
	})

	t.Run("an empty value disables the filter", func(t *testing.T) {
		t.Parallel()
		var gotCmd application.GlobalOperationsListQuery
		svc := &fakeOperationsManager{
			globalList: func(
				_ context.Context, _ uuid.UUID, cmd application.GlobalOperationsListQuery,
			) (application.GlobalOperationsPage, error) {
				gotCmd = cmd
				return application.GlobalOperationsPage{}, nil
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())
		empty := " , "

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
		)
		w := httptest.NewRecorder()
		h.ListOperations(w, req, openapi.ListOperationsParams{PropertyIds: &empty})

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if gotCmd.PropertyIDs != nil {
			t.Errorf("cmd.PropertyIDs = %v, want nil — the merged feed", gotCmd.PropertyIDs)
		}
	})

	t.Run("a foreign filter id is the privacy 404", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			globalList: func(context.Context, uuid.UUID, application.GlobalOperationsListQuery) (application.GlobalOperationsPage, error) {
				return application.GlobalOperationsPage{}, application.ErrNotFound
			},
		}
		h := NewOperationsHandlers(svc, nil)
		actor := uuid.Must(uuid.NewV7())

		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations", nil,
		)
		w := httptest.NewRecorder()
		h.ListOperations(w, req, openapi.ListOperationsParams{})

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want the privacy 404", w.Code)
		}
	})
}

func TestSummarizeOperations_FoldsParamsAndMapsPrivacy(t *testing.T) {
	t.Parallel()

	propA := uuid.Must(uuid.NewV7())
	var gotCmd application.GlobalOperationsSummaryQuery
	svc := &fakeOperationsManager{
		globalSummarize: func(
			_ context.Context, _ uuid.UUID, cmd application.GlobalOperationsSummaryQuery,
		) (application.OperationsSummary, error) {
			gotCmd = cmd
			return application.OperationsSummary{
				IncomeTotalKopecks:  5650000,
				ExpenseTotalKopecks: 250000,
				Categories: []application.CategorySummary{
					{Slug: testSlugRent, Label: testLabelRent, Type: domain.TypeIncome, TotalKopecks: 5650000},
				},
			}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	propertyIds := " " + propA.String() + " "
	income := openapi.SummarizeOperationsParamsTypeIncome
	from := openapi_types.Date{Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodGet, "/operations/summary", nil,
	)
	w := httptest.NewRecorder()
	h.SummarizeOperations(w, req, openapi.SummarizeOperationsParams{
		PropertyIds: &propertyIds,
		Type:        &income,
		DateFrom:    &from,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if len(gotCmd.PropertyIDs) != 1 || gotCmd.PropertyIDs[0] != propA {
		t.Errorf("cmd.PropertyIDs = %v, want [%s]", gotCmd.PropertyIDs, propA)
	}
	if gotCmd.Type == nil || *gotCmd.Type != domain.TypeIncome {
		t.Errorf("cmd.Type = %v, want income", gotCmd.Type)
	}
	if gotCmd.DateFrom == nil || gotCmd.DateTo != nil {
		t.Errorf("period = %v..%v, want the from bound only", gotCmd.DateFrom, gotCmd.DateTo)
	}

	var body openapi.OperationsSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.IncomeTotalKopecks != 5650000 || len(body.Categories) != 1 {
		t.Errorf("wire summary = %+v, want the totals with the single rent chip", body)
	}

	t.Run("a non-uuid entry is 400", func(t *testing.T) {
		t.Parallel()
		h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
		bogus := "123"
		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), uuid.Must(uuid.NewV7())), http.MethodGet, "/", nil,
		)
		w := httptest.NewRecorder()
		h.SummarizeOperations(w, req, openapi.SummarizeOperationsParams{PropertyIds: &bogus})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for a non-uuid propertyIds entry", w.Code)
		}
	})

	t.Run("a foreign filter id is the privacy 404", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			globalSummarize: func(context.Context, uuid.UUID, application.GlobalOperationsSummaryQuery) (application.OperationsSummary, error) {
				return application.OperationsSummary{}, application.ErrNotFound
			},
		}
		h := NewOperationsHandlers(svc, nil)
		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), uuid.Must(uuid.NewV7())), http.MethodGet, "/", nil,
		)
		w := httptest.NewRecorder()
		h.SummarizeOperations(w, req, openapi.SummarizeOperationsParams{})
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want the privacy 404", w.Code)
		}
	})
}

func TestCreateOperation_FoldsBodyIntoCommandAndReturns201(t *testing.T) {
	t.Parallel()

	created := fixtureOperation(domain.StatusPaid)
	created.Origin = domain.OriginManual
	created.PaymentID = nil
	created.PaymentForm = nil

	var gotCmd application.CreateOperationCommand
	var gotProperty uuid.UUID
	svc := &fakeOperationsManager{
		create: func(
			_ context.Context, _, propertyID uuid.UUID, cmd application.CreateOperationCommand,
		) (application.OperationListItem, error) {
			gotCmd = cmd
			gotProperty = propertyID
			return application.OperationListItem{Operation: created, ViewStatus: domain.ViewStatusPaid}, nil
		},
	}
	h := NewOperationsHandlers(svc, nil)
	propertyID := uuid.Must(uuid.NewV7())
	actor := uuid.Must(uuid.NewV7())

	body := `{"type":"expense","title":"Ремонт крана","amountKopecks":150000,"categorySlug":"utilities"}`
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor), http.MethodPost, "/operations", strings.NewReader(body),
	)
	w := httptest.NewRecorder()
	h.CreateOperation(w, req, propertyID)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	if gotProperty != propertyID {
		t.Errorf("property id = %s, want %s", gotProperty, propertyID)
	}
	want := application.CreateOperationCommand{
		Type: domain.TypeExpense, Title: "Ремонт крана", AmountKopecks: 150000, CategorySlug: testSlugUtilities,
	}
	if gotCmd != want {
		t.Errorf("command = %+v, want %+v", gotCmd, want)
	}
	var resp openapi.OperationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != openapi.OperationResponseStatusPaid {
		t.Errorf("status = %q, want the born-paid fact", resp.Status)
	}
	if resp.PaymentId != nil || resp.PaymentForm != nil {
		t.Errorf("paymentId/paymentForm = %v/%v, want both null behind a manual fact", resp.PaymentId, resp.PaymentForm)
	}
}

func TestCreateOperation_ErrorMapping(t *testing.T) {
	t.Parallel()
	actor := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	payload := `{"type":"expense","title":"ЖКУ","amountKopecks":500000,"categorySlug":"utilities"}`

	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"invalid input is the contract 400", application.ErrInvalidInput, http.StatusBadRequest},
		{"a viewer is the 403", application.ErrForbidden, http.StatusForbidden},
		{"a stranger is the privacy 404", application.ErrNotFound, http.StatusNotFound},
		{"an archived property is the 409", application.ErrArchivedProperty, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeOperationsManager{
				create: func(context.Context, uuid.UUID, uuid.UUID, application.CreateOperationCommand) (application.OperationListItem, error) {
					return application.OperationListItem{}, tc.err
				},
			}
			h := NewOperationsHandlers(svc, nil)
			req := httptest.NewRequestWithContext(
				httpsupport.WithUserID(t.Context(), actor), http.MethodPost, "/operations", strings.NewReader(payload),
			)
			w := httptest.NewRecorder()
			h.CreateOperation(w, req, propertyID)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}

	t.Run("a broken body is the 400", func(t *testing.T) {
		t.Parallel()
		h := NewOperationsHandlers(&fakeOperationsManager{}, nil)
		req := httptest.NewRequestWithContext(
			httpsupport.WithUserID(t.Context(), actor), http.MethodPost, "/operations", strings.NewReader("{not json"),
		)
		w := httptest.NewRecorder()
		h.CreateOperation(w, req, propertyID)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
