package http

// The wire contract tests of the rentals handlers: the decode and encode
// round-trips over func-backed fakes (the payments handler-test pattern).
// The application behaviour behind the port is covered by the application
// unit and integration families; here only the wire mapping matters.

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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRentalManager is the func-backed RentalManager double: every use case
// is optional; an unset one fails the test loudly instead of silently
// succeeding. The func fields back the port's methods one to one; the field
// order is alphabetical, unlike the port.
type fakeRentalManager struct {
	complete func(
		ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd rentalsapp.CompleteRentalCommand,
	) (rentalsapp.RentalView, error)
	create  func(ctx context.Context, actor, propertyID uuid.UUID, cmd rentalsapp.CreateRentalCommand) (rentalsapp.RentalView, error)
	del     func(ctx context.Context, actor, propertyID, rentalID uuid.UUID) error
	get     func(ctx context.Context, actor, propertyID, rentalID uuid.UUID) (rentalsapp.RentalView, error)
	list    func(ctx context.Context, actor, propertyID uuid.UUID) ([]rentalsapp.RentalView, error)
	summary func(ctx context.Context, actor, propertyID, rentalID uuid.UUID, until *time.Time) (rentalsapp.RentalSummary, error)
	update  func(
		ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd rentalsapp.UpdateRentalCommand,
	) (rentalsapp.RentalView, error)
}

func (f *fakeRentalManager) CreateRental(
	ctx context.Context, actor, propertyID uuid.UUID, cmd rentalsapp.CreateRentalCommand,
) (rentalsapp.RentalView, error) {
	if f.create == nil {
		return rentalsapp.RentalView{}, errors.New("unexpected CreateRental call")
	}
	return f.create(ctx, actor, propertyID, cmd)
}

func (f *fakeRentalManager) ListRentals(
	ctx context.Context, actor, propertyID uuid.UUID,
) ([]rentalsapp.RentalView, error) {
	if f.list == nil {
		return nil, errors.New("unexpected ListRentals call")
	}
	return f.list(ctx, actor, propertyID)
}

func (f *fakeRentalManager) GetRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID,
) (rentalsapp.RentalView, error) {
	if f.get == nil {
		return rentalsapp.RentalView{}, errors.New("unexpected GetRental call")
	}
	return f.get(ctx, actor, propertyID, rentalID)
}

func (f *fakeRentalManager) UpdateRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd rentalsapp.UpdateRentalCommand,
) (rentalsapp.RentalView, error) {
	if f.update == nil {
		return rentalsapp.RentalView{}, errors.New("unexpected UpdateRental call")
	}
	return f.update(ctx, actor, propertyID, rentalID, cmd)
}

func (f *fakeRentalManager) CompleteRental(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd rentalsapp.CompleteRentalCommand,
) (rentalsapp.RentalView, error) {
	if f.complete == nil {
		return rentalsapp.RentalView{}, errors.New("unexpected CompleteRental call")
	}
	return f.complete(ctx, actor, propertyID, rentalID, cmd)
}

func (f *fakeRentalManager) DeleteRental(ctx context.Context, actor, propertyID, rentalID uuid.UUID) error {
	if f.del == nil {
		return errors.New("unexpected DeleteRental call")
	}
	return f.del(ctx, actor, propertyID, rentalID)
}

func (f *fakeRentalManager) RentalSummary(
	ctx context.Context, actor, propertyID, rentalID uuid.UUID, until *time.Time,
) (rentalsapp.RentalSummary, error) {
	if f.summary == nil {
		return rentalsapp.RentalSummary{}, errors.New("unexpected RentalSummary call")
	}
	return f.summary(ctx, actor, propertyID, rentalID, until)
}

// rentalRequest builds an authenticated handler request with a JSON body.
func rentalRequest(t *testing.T, method string, userID uuid.UUID, target, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), userID),
		method, target, strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// wireDate builds the wire date value.
func wireDate(s string) openapi_types.Date {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return openapi_types.Date{Time: t}
}

// viewFixture is the assembled view the fakes answer with.
func viewFixture(t *testing.T) rentalsapp.RentalView {
	t.Helper()
	day := rentalsdomain.MustPaymentDay(15)
	paymentID := uuid.Must(uuid.NewV7())
	return rentalsapp.RentalView{
		Rental: rentalsdomain.Rental{
			ID:         uuid.Must(uuid.NewV7()),
			OwnerID:    uuid.Must(uuid.NewV7()),
			PropertyID: uuid.Must(uuid.NewV7()),
			PaymentID:  &paymentID,
			StartDate:  wireDate("2026-09-01").Time,
			PlannedEndDate: func() *time.Time {
				d := wireDate("2027-09-01").Time
				return &d
			}(),
			Utilities: rentalsdomain.UtilitiesIncluded,
			Comment:   "нюансы",
			Tenant: &rentalsdomain.TenantContact{
				ContactID: uuid.Must(uuid.NewV7()),
				FirstName: "Иван",
				LastName:  "Иванов",
				Phone:     "+79990000000",
			},
		},
		Status: rentalsdomain.StatusActive,
		Payment: rentalsapp.RentPaymentState{
			PaymentID:          paymentID,
			AmountKopecks:      5_000_000,
			PaymentDay:         day,
			AutoPay:            true,
			ReminderOffsetDays: func() *int { v := 3; return &v }(),
		},
		NextPayment: &rentalsapp.PlannedOccurrence{
			OperationID:   uuid.Must(uuid.NewV7()),
			Date:          wireDate("2026-09-15").Time,
			AmountKopecks: 5_000_000,
		},
		Progress: rentalsapp.RentalProgress{
			PaidMonths:      0,
			TotalMonths:     func() *int { v := 12; return &v }(),
			MonthsRemaining: func() *int { v := 11; return &v }(),
			OverdueMonths:   func() *int { v := 2; return &v }(),
		},
		Today: wireDate("2026-09-04").Time,
	}
}

func TestCreateRental_Created(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotCmd rentalsapp.CreateRentalCommand
	svc := &fakeRentalManager{create: func(
		_ context.Context, actor, propertyID uuid.UUID, cmd rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotCmd = cmd
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	body := `{
		"amountKopecks": 5000000,
		"paymentDay": 15,
		"startDate": "2026-09-04",
		"plannedEndDate": "2027-09-01",
		"utilities": "included",
		"depositKopecks": 10000000,
		"contactId": "%s",
		"comment": "нюансы",
		"autoPay": true,
		"reminderOffsetDays": 3
	}`
	contact := uuid.Must(uuid.NewV7())
	req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), fmt.Sprintf(body, contact))
	rec := httptest.NewRecorder()

	h.CreateRental(rec, req, view.Rental.PropertyID)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, int64(5_000_000), gotCmd.AmountKopecks)
	assert.Equal(t, 15, gotCmd.PaymentDay.Day())
	require.NotNil(t, gotCmd.DepositKopecks)
	assert.Equal(t, int64(10_000_000), *gotCmd.DepositKopecks)
	require.NotNil(t, gotCmd.ContactID)
	assert.Equal(t, contact, *gotCmd.ContactID)
	assert.Equal(t, "2026-09-04", gotCmd.StartDate.Format(time.DateOnly))
	assert.True(t, gotCmd.AutoPay)
	require.NotNil(t, gotCmd.ReminderOffsetDays)
	assert.Equal(t, 3, *gotCmd.ReminderOffsetDays)

	var response openapi.RentalResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, view.Rental.ID, response.Id)
	assert.Equal(t, openapi.RentalResponseStatus(rentalsdomain.StatusActive), response.Status)
	assert.Equal(t, view.Rental.PaymentID, response.RentPayment.PaymentId)
	assert.Equal(t, int64(5_000_000), response.RentPayment.AmountKopecks)
	assert.Equal(t, "2026-09-15", response.RentPayment.NextPayment.Date.Format(time.DateOnly))
	assert.Equal(t, 11, response.RentPayment.NextPayment.DaysUntil)
	require.NotNil(t, response.RentPayment.ReminderOffsetDays)
	assert.Equal(t, openapi.RentalPaymentViewReminderOffsetDays(3), *response.RentPayment.ReminderOffsetDays)
	require.NotNil(t, response.Tenant)
	assert.Equal(t, "Иван", response.Tenant.FirstName)
	require.NotNil(t, response.Progress.TotalMonths)
	assert.Equal(t, 12, *response.Progress.TotalMonths)
	require.NotNil(t, response.Progress.OverdueMonths,
		"the server-counted overdue rides the progress node (#817)")
	assert.Equal(t, 2, *response.Progress.OverdueMonths)
	assert.Equal(t, "2026-09-04", response.Today.Format(time.DateOnly))
}

// TestCreateRental_NoReminderOffset pins the nil leg of reminderOffsetDays
// (the payments wire-test «absent — no reminders» case mirrored): a body
// without the field decodes to no command pointer, and a view without the
// leg renders no reminderOffsetDays key back to the wire.
func TestCreateRental_NoReminderOffset(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	view.Payment.ReminderOffsetDays = nil
	var gotCmd rentalsapp.CreateRentalCommand
	svc := &fakeRentalManager{create: func(
		_ context.Context, _, _ uuid.UUID, cmd rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotCmd = cmd
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	body := `{"amountKopecks": 5000000, "paymentDay": 15, "startDate": "2026-09-04",
	          "utilities": "meters_only", "autoPay": false}`
	req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), body)
	rec := httptest.NewRecorder()

	h.CreateRental(rec, req, view.Rental.PropertyID)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Nil(t, gotCmd.ReminderOffsetDays)
	assert.NotContains(t, rec.Body.String(), "reminderOffsetDays",
		"a view without the leg must render no reminderOffsetDays key")
}

func TestCreateRental_PaymentDayLast(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	view.Payment.PaymentDay = rentalsdomain.NewLastPaymentDay()
	var gotDay rentalsdomain.PaymentDay
	svc := &fakeRentalManager{create: func(
		_ context.Context, _, _ uuid.UUID, cmd rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotDay = cmd.PaymentDay
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	body := `{"amountKopecks": 5000000, "paymentDay": "last", "startDate": "2026-09-04",
	          "utilities": "meters_only", "autoPay": false}`
	req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), body)
	rec := httptest.NewRecorder()

	h.CreateRental(rec, req, view.Rental.PropertyID)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, gotDay.IsLast(), "the wire «last» marker must decode to the last-day behaviour")

	// And the response renders the marker back as "last".
	var response openapi.RentalResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	marker, err := response.RentPayment.PaymentDay.AsRentalPaymentDay1()
	require.NoError(t, err)
	assert.Equal(t, "last", string(marker))
}

func TestCreateRental_PaymentDay31IsLast(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotDay rentalsdomain.PaymentDay
	svc := &fakeRentalManager{create: func(
		_ context.Context, _, _ uuid.UUID, cmd rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotDay = cmd.PaymentDay
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	body := `{"amountKopecks": 5000000, "paymentDay": 31, "startDate": "2026-09-04",
	          "utilities": "full_receipt", "autoPay": false}`
	req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), body)
	rec := httptest.NewRecorder()

	h.CreateRental(rec, req, view.Rental.PropertyID)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, gotDay.IsLast(), "31 is the one last-day behaviour (решение №5)")
}

func TestErrorMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"not found", rentalsapp.ErrNotFound, http.StatusNotFound},
		{"forbidden", rentalsapp.ErrForbidden, http.StatusForbidden},
		{"archived property", rentalsapp.ErrArchivedProperty, http.StatusConflict},
		{"occupied property", rentalsapp.ErrPropertyOccupied, http.StatusConflict},
		{"maintenance property", rentalsapp.ErrPropertyMaintenance, http.StatusConflict},
		{"completed rental", rentalsapp.ErrRentalCompleted, http.StatusConflict},
		{"started rental", rentalsapp.ErrRentalStarted, http.StatusConflict},
		{"invalid input", rentalsapp.ErrInvalidInput, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeRentalManager{get: func(
				_ context.Context, _, _, _ uuid.UUID,
			) (rentalsapp.RentalView, error) {
				return rentalsapp.RentalView{}, tt.err
			}}
			h := NewRentalHandlers(svc, nil)
			id := uuid.Must(uuid.NewV7())
			req := rentalRequest(t, http.MethodGet, id, "/", "")
			rec := httptest.NewRecorder()

			h.GetRental(rec, req, id, id)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if errors.Is(tt.err, rentalsapp.ErrInvalidInput) {
				assert.Contains(t, rec.Body.String(), "Некорректные данные аренды")
			}
		})
	}
}

// The maintenance guard is the context's only coded conflict (ticket #1050):
// the machine-readable code rides the problem body — the shape the client
// distinguishes the guard by, canon `membership_suspended`/`property_occupied`.
func TestErrorMapping_MaintenanceCarriesCode(t *testing.T) {
	t.Parallel()
	svc := &fakeRentalManager{create: func(
		_ context.Context, _, _ uuid.UUID, _ rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error) {
		return rentalsapp.RentalView{}, rentalsapp.ErrPropertyMaintenance
	}}
	h := NewRentalHandlers(svc, nil)
	id := uuid.Must(uuid.NewV7())
	req := rentalRequest(t, http.MethodPost, id, "/",
		`{"amountKopecks": 5000000, "paymentDay": 15, "startDate": "2026-09-04",
		  "utilities": "included", "autoPay": false}`)
	rec := httptest.NewRecorder()

	h.CreateRental(rec, req, id)

	require.Equal(t, http.StatusConflict, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"code":"property_maintenance"`)
	assert.Contains(t, body, "Объект на ремонте — аренда недоступна")
}

func TestUpdateRental_TriState(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotCmd rentalsapp.UpdateRentalCommand
	svc := &fakeRentalManager{update: func(
		_ context.Context, _, _, _ uuid.UUID, cmd rentalsapp.UpdateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotCmd = cmd
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	// Amount + day set; plannedEndDate and contactId explicit nulls (clear);
	// deposit, commission, comment omitted (keep).
	body := `{"amountKopecks": 6000000, "paymentDay": "last",
	          "plannedEndDate": null, "contactId": null}`
	req := rentalRequest(t, http.MethodPatch, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals/%s", view.Rental.PropertyID, view.Rental.ID), body)
	rec := httptest.NewRecorder()

	h.UpdateRental(rec, req, view.Rental.PropertyID, view.Rental.ID)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotCmd.AmountKopecks)
	assert.Equal(t, int64(6_000_000), *gotCmd.AmountKopecks)
	require.NotNil(t, gotCmd.PaymentDay)
	assert.True(t, gotCmd.PaymentDay.IsLast())

	require.NotNil(t, gotCmd.PlannedEndDate, "an explicit null is the clear verdict")
	assert.Nil(t, gotCmd.PlannedEndDate.Value)
	require.NotNil(t, gotCmd.ContactID)
	assert.Nil(t, gotCmd.ContactID.Value)

	assert.Nil(t, gotCmd.DepositKopecks, "omitted keeps")
	assert.Nil(t, gotCmd.CommissionKopecks, "omitted keeps")
	assert.Nil(t, gotCmd.Comment, "omitted keeps")
	assert.Nil(t, gotCmd.Utilities, "omitted keeps")
	assert.Nil(t, gotCmd.NotifyEmail, "omitted keeps")
}

// The email-toggle create mapping (карта #1551, тикет #1555): omitted
// resolves the default true («включена», как у обычных платежей #1553), an
// explicit false/true rides verbatim; the response renders the flag from the
// payment state — never from the request (the fixture's state disagrees with
// the sent value on purpose, so every case doubles as the independence pin).
func TestCreateRental_NotifyEmailMaps(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		field       string
		wantCmd     bool
		renderState bool
	}{
		{"omitted resolves the default true", "", true, false},
		{"explicit false rides false", `"notifyEmail": false`, false, true},
		{"explicit true rides true", `"notifyEmail": true`, true, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			view := viewFixture(t)
			view.Payment.NotifyEmail = tc.renderState
			var gotCmd rentalsapp.CreateRentalCommand
			svc := &fakeRentalManager{create: func(
				_ context.Context, _, _ uuid.UUID, cmd rentalsapp.CreateRentalCommand,
			) (rentalsapp.RentalView, error) {
				gotCmd = cmd
				return view, nil
			}}
			h := NewRentalHandlers(svc, nil)

			fields := `"amountKopecks": 5000000, "paymentDay": 15, "startDate": "2026-09-04",
			           "utilities": "meters_only", "autoPay": false`
			if tc.field != "" {
				fields += ", " + tc.field
			}
			req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
				fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), "{"+fields+"}")
			rec := httptest.NewRecorder()

			h.CreateRental(rec, req, view.Rental.PropertyID)

			require.Equal(t, http.StatusCreated, rec.Code)
			assert.Equal(t, tc.wantCmd, gotCmd.NotifyEmail)

			var response openapi.RentalResponse
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
			assert.Equal(t, tc.renderState, response.RentPayment.NotifyEmail,
				"the flag renders from the payment state, not the request")
		})
	}
}

// The completed rental's wire shape (ревизия #1161, тикет #1555): the
// payment is deleted and the terms archive does not store the email flag —
// the response carries false beside the null paymentId.
func TestCreateRental_CompletedRentalRendersNoNotifyEmail(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	view.Payment.PaymentID = uuid.Nil
	view.Payment.NotifyEmail = false
	svc := &fakeRentalManager{create: func(
		_ context.Context, _, _ uuid.UUID, _ rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error) {
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	body := `{"amountKopecks": 5000000, "paymentDay": 15, "startDate": "2026-09-04",
	          "utilities": "meters_only", "autoPay": false, "notifyEmail": true}`
	req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), body)
	rec := httptest.NewRecorder()

	h.CreateRental(rec, req, view.Rental.PropertyID)

	require.Equal(t, http.StatusCreated, rec.Code)
	var response openapi.RentalResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Nil(t, response.RentPayment.PaymentId)
	assert.False(t, response.RentPayment.NotifyEmail,
		"the archive stores no email flag — even a sent true renders false")
}

// The email-toggle patch's tri-state (тикет #1555): true/false set the flag,
// an explicit null turns the emails off (null ≡ false — колонка не-nullable),
// an omitted field keeps — the same resolution vocabulary as the payment's
// own PATCH (#1553).
func TestUpdateRental_NotifyEmailTriState(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotCmds []rentalsapp.UpdateRentalCommand
	svc := &fakeRentalManager{update: func(
		_ context.Context, _, _, _ uuid.UUID, cmd rentalsapp.UpdateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotCmds = append(gotCmds, cmd)
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)
	target := fmt.Sprintf("/properties/%s/rentals/%s", view.Rental.PropertyID, view.Rental.ID)

	patch := func(t *testing.T, body string) {
		t.Helper()
		req := rentalRequest(t, http.MethodPatch, view.Rental.OwnerID, target, body)
		rec := httptest.NewRecorder()
		h.UpdateRental(rec, req, view.Rental.PropertyID, view.Rental.ID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	patch(t, `{"notifyEmail": true}`)
	require.Len(t, gotCmds, 1)
	require.NotNil(t, gotCmds[0].NotifyEmail, "a value is the set verdict")
	assert.True(t, *gotCmds[0].NotifyEmail)

	patch(t, `{"notifyEmail": false}`)
	require.Len(t, gotCmds, 2)
	require.NotNil(t, gotCmds[1].NotifyEmail)
	assert.False(t, *gotCmds[1].NotifyEmail)

	patch(t, `{"notifyEmail": null}`)
	require.Len(t, gotCmds, 3)
	require.NotNil(t, gotCmds[2].NotifyEmail, "an explicit null is the off verdict")
	assert.False(t, *gotCmds[2].NotifyEmail)

	patch(t, `{"amountKopecks": 6000000}`)
	require.Len(t, gotCmds, 4)
	assert.Nil(t, gotCmds[3].NotifyEmail, "omitted keeps")
}

func TestUpdateRental_UnknownFieldIs400(t *testing.T) {
	t.Parallel()
	svc := &fakeRentalManager{}
	h := NewRentalHandlers(svc, nil)
	id := uuid.Must(uuid.NewV7())

	req := rentalRequest(t, http.MethodPatch, id, "/", `{"startDate": "2026-09-01"}`)
	rec := httptest.NewRecorder()
	h.UpdateRental(rec, req, id, id)

	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"the start date is not editable and must not sneak in as an unknown field")
}

// The reminder patch's tri-state (#1208): a value sets the lead time, an
// explicit null turns the reminders off, an omitted field keeps — the same
// resolution vocabulary as the payment's own PATCH (#1197).
func TestUpdateRental_ReminderOffsetTriState(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotCmds []rentalsapp.UpdateRentalCommand
	svc := &fakeRentalManager{update: func(
		_ context.Context, _, _, _ uuid.UUID, cmd rentalsapp.UpdateRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotCmds = append(gotCmds, cmd)
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)
	target := fmt.Sprintf("/properties/%s/rentals/%s", view.Rental.PropertyID, view.Rental.ID)

	patch := func(t *testing.T, body string) {
		t.Helper()
		req := rentalRequest(t, http.MethodPatch, view.Rental.OwnerID, target, body)
		rec := httptest.NewRecorder()
		h.UpdateRental(rec, req, view.Rental.PropertyID, view.Rental.ID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	patch(t, `{"reminderOffsetDays": 7}`)
	require.Len(t, gotCmds, 1)
	require.NotNil(t, gotCmds[0].ReminderOffsetDays, "a value is the set verdict")
	require.NotNil(t, gotCmds[0].ReminderOffsetDays.Value)
	assert.Equal(t, 7, *gotCmds[0].ReminderOffsetDays.Value)

	patch(t, `{"reminderOffsetDays": null}`)
	require.Len(t, gotCmds, 2)
	require.NotNil(t, gotCmds[1].ReminderOffsetDays, "an explicit null is the clear verdict")
	assert.Nil(t, gotCmds[1].ReminderOffsetDays.Value)

	patch(t, `{"amountKopecks": 6000000}`)
	require.Len(t, gotCmds, 3)
	assert.Nil(t, gotCmds[2].ReminderOffsetDays, "omitted keeps")
}

func TestCompleteRental_DecodesDepositReturn(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotCmd rentalsapp.CompleteRentalCommand
	svc := &fakeRentalManager{complete: func(
		_ context.Context, _, _, _ uuid.UUID, cmd rentalsapp.CompleteRentalCommand,
	) (rentalsapp.RentalView, error) {
		gotCmd = cmd
		return view, nil
	}}
	h := NewRentalHandlers(svc, nil)

	body := `{"completedDate": "2026-09-01",
	          "depositReturn": {"amountKopecks": 9000000, "comment": "частично"}}`
	req := rentalRequest(t, http.MethodPost, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals/%s/complete", view.Rental.PropertyID, view.Rental.ID), body)
	rec := httptest.NewRecorder()

	h.CompleteRental(rec, req, view.Rental.PropertyID, view.Rental.ID)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "2026-09-01", gotCmd.CompletedDate.Format(time.DateOnly))
	require.NotNil(t, gotCmd.DepositReturn)
	assert.Equal(t, int64(9_000_000), gotCmd.DepositReturn.AmountKopecks)
	require.NotNil(t, gotCmd.DepositReturn.Comment)
	assert.Equal(t, "частично", *gotCmd.DepositReturn.Comment)
}

func TestGetRentalSummary_UntilParam(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var gotUntil *time.Time
	svc := &fakeRentalManager{summary: func(
		_ context.Context, _, _, _ uuid.UUID, until *time.Time,
	) (rentalsapp.RentalSummary, error) {
		gotUntil = until
		return rentalsapp.RentalSummary{
			From:           wireDate("2026-09-01").Time,
			Until:          wireDate("2026-09-04").Time,
			IncomeKopecks:  10_000_000,
			ExpenseKopecks: 3_000_000,
			ProfitKopecks:  7_000_000,
		}, nil
	}}
	h := NewRentalHandlers(svc, nil)

	target := fmt.Sprintf("/properties/%s/rentals/%s/summary?until=2026-09-03",
		view.Rental.PropertyID, view.Rental.ID)
	req := rentalRequest(t, http.MethodGet, view.Rental.OwnerID, target, "")
	rec := httptest.NewRecorder()
	h.GetRentalSummary(rec, req, view.Rental.PropertyID, view.Rental.ID,
		openapi.GetRentalSummaryParams{Until: func() *openapi_types.Date {
			d := wireDate("2026-09-03")
			return &d
		}()})

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotUntil)
	assert.Equal(t, "2026-09-03", gotUntil.Format(time.DateOnly))

	var summary openapi.RentalSummaryResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&summary))
	assert.Equal(t, int64(7_000_000), summary.ProfitKopecks)
}

func TestDeleteRental_NoContent(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	var deleted uuid.UUID
	svc := &fakeRentalManager{del: func(
		_ context.Context, _, _, rentalID uuid.UUID,
	) error {
		deleted = rentalID
		return nil
	}}
	h := NewRentalHandlers(svc, nil)

	req := rentalRequest(t, http.MethodDelete, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals/%s", view.Rental.PropertyID, view.Rental.ID), "")
	rec := httptest.NewRecorder()
	h.DeleteRental(rec, req, view.Rental.PropertyID, view.Rental.ID)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, view.Rental.ID, deleted)
}

// Ревизия #1161: у завершённой аренды платёж удалён — paymentId уходит в
// wire как null, условия рендерятся из Архива условий.
func TestListRentals_CompletedCarriesNullPaymentId(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	view.Status = rentalsdomain.StatusCompleted
	view.Rental.CompletedDate = &view.Today
	view.Rental.PaymentID = nil
	view.Rental.TermsArchive = &rentalsdomain.TermsArchive{
		AmountKopecks: 5_000_000,
		PaymentDay:    rentalsdomain.MustPaymentDay(15),
		AutoPay:       true,
	}
	view.Payment = rentalsapp.RentPaymentState{
		AmountKopecks: 5_000_000,
		PaymentDay:    rentalsdomain.MustPaymentDay(15),
		AutoPay:       true,
	}
	view.NextPayment = nil
	svc := &fakeRentalManager{list: func(
		_ context.Context, _, _ uuid.UUID,
	) ([]rentalsapp.RentalView, error) {
		return []rentalsapp.RentalView{view}, nil
	}}
	h := NewRentalHandlers(svc, nil)

	req := rentalRequest(t, http.MethodGet, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), "")
	rec := httptest.NewRecorder()
	h.ListRentals(rec, req, view.Rental.PropertyID)
	require.Equal(t, http.StatusOK, rec.Code)

	raw := rec.Body.String()
	var response openapi.RentalsResponse
	require.NoError(t, json.NewDecoder(strings.NewReader(raw)).Decode(&response))
	require.Len(t, response.Items, 1)
	require.Nil(t, response.Items[0].RentPayment.PaymentId,
		"the deleted payment's id is the wire's null")
	assert.Equal(t, int64(5_000_000), response.Items[0].RentPayment.AmountKopecks,
		"the archived amount")
	assert.Nil(t, response.Items[0].RentPayment.NextPayment)

	// И сам JSON несёт явный null — поле required (не omit).
	assert.Contains(t, raw, `"paymentId":null`)
}

func TestListRentals_Items(t *testing.T) {
	t.Parallel()
	view := viewFixture(t)
	svc := &fakeRentalManager{list: func(
		_ context.Context, _, _ uuid.UUID,
	) ([]rentalsapp.RentalView, error) {
		return []rentalsapp.RentalView{view}, nil
	}}
	h := NewRentalHandlers(svc, nil)

	req := rentalRequest(t, http.MethodGet, view.Rental.OwnerID,
		fmt.Sprintf("/properties/%s/rentals", view.Rental.PropertyID), "")
	rec := httptest.NewRecorder()
	h.ListRentals(rec, req, view.Rental.PropertyID)

	require.Equal(t, http.StatusOK, rec.Code)
	var response openapi.RentalsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	require.Len(t, response.Items, 1)
}

func TestUnauthorizedIs401(t *testing.T) {
	t.Parallel()
	h := NewRentalHandlers(&fakeRentalManager{}, nil)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	h.CreateRental(rec, req, uuid.Must(uuid.NewV7()))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
