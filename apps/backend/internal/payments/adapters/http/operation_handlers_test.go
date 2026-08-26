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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// fakeOperationsManager is the func-backed OperationsManager double.
type fakeOperationsManager struct {
	pay   func(ctx context.Context, actor, propertyID, operationID uuid.UUID) (domain.Operation, error)
	byPay func(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
	byProp func(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
}

func (f *fakeOperationsManager) PayOperation(
	ctx context.Context, actor, propertyID, operationID uuid.UUID,
) (domain.Operation, error) {
	if f.pay == nil {
		return domain.Operation{}, errors.New("unexpected PayOperation call")
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

	status := openapi.Overdue
	from := openapi_types.Date{Time: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	to := openapi_types.Date{Time: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}
	asc := openapi.Asc
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
	if gotCmd.Desc {
		t.Error("order=asc lost: Desc = true, want false")
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
	if body.Items[0].Status != openapi.OperationResponseStatus(openapi.Overdue) {
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
	if !gotCmd.Desc {
		t.Error("default direction = asc, want desc")
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

	planned := openapi.ListPropertyOperationsParamsStatus(openapi.Planned)
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
	if len(items.Items) != 1 || items.Items[0].Status != openapi.OperationResponseStatus(openapi.Planned) {
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

func TestPayOperation_HappyPathAndConflicts(t *testing.T) {
	t.Parallel()

	paidOp := fixtureOperation(domain.StatusPaid)

	t.Run("returns the paid operation", func(t *testing.T) {
		t.Parallel()
		var gotID uuid.UUID
		svc := &fakeOperationsManager{
			pay: func(_ context.Context, _, _, operationID uuid.UUID) (domain.Operation, error) {
				gotID = operationID
				return paidOp, nil
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
		if body.Status != openapi.OperationResponseStatus(openapi.Paid) {
			t.Errorf("status = %q, want paid", body.Status)
		}
		if body.PaidDate == nil || !body.PaidDate.Equal(paidOp.PaidDate.UTC()) {
			t.Errorf("paidDate = %v, want %v", body.PaidDate, paidOp.PaidDate)
		}
	})

	t.Run("a repeated pay maps to 409", func(t *testing.T) {
		t.Parallel()
		svc := &fakeOperationsManager{
			pay: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Operation, error) {
				return domain.Operation{}, application.ErrAlreadyPaid
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
			pay: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Operation, error) {
				return domain.Operation{}, application.ErrForbidden
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
