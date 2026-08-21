package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	domain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// newPropertyHandlersForMapping builds a PropertyHandlers with only the fields
// that propertyResponse and handlePropertyError touch. The concrete service is
// left nil; these mapping helpers do not call into it.
func newPropertyHandlersForMapping(t *testing.T) *PropertyHandlers {
	t.Helper()
	return &PropertyHandlers{logger: slog.New(slog.DiscardHandler)}
}

// attrFloor — код атрибута, повторённый маппинг-тестами пакета.
const attrFloor = "floor"

func TestPropertyResponse_IncludesAttributes(t *testing.T) {
	t.Parallel()

	h := newPropertyHandlersForMapping(t)

	property := domain.Property{
		ID:         uuid.Must(uuid.NewV7()),
		OwnerID:    uuid.Must(uuid.NewV7()),
		Name:       "Flat",
		Type:       domain.PropertyTypeApartment,
		Address:    " ul. Pushkina",
		Status:     domain.PropertyStatusActive,
		Attributes: domain.Attributes{"rooms": "2", "area_total": 50.0},
	}

	resp, err := h.propertyResponse(context.Background(), property.OwnerID, property, leasesdomain.Lease{})
	if err != nil {
		t.Fatalf("propertyResponse error: %v", err)
	}

	if resp.Attributes == nil {
		t.Fatal("resp.Attributes = nil, want non-nil map")
	}
	if got := len(resp.Attributes); got != 2 {
		t.Fatalf("len(resp.Attributes) = %d, want 2", got)
	}
	if got, ok := resp.Attributes["rooms"]; !ok || got != "2" {
		t.Fatalf("resp.Attributes[rooms] = %v (ok=%v), want \"2\"", got, ok)
	}
	if got, ok := resp.Attributes["area_total"]; !ok || got != 50.0 {
		t.Fatalf("resp.Attributes[area_total] = %v (ok=%v), want 50.0", got, ok)
	}
}

func TestPropertyResponse_EmptyAttributesWhenNil(t *testing.T) {
	t.Parallel()

	h := newPropertyHandlersForMapping(t)

	property := domain.Property{
		ID:         uuid.Must(uuid.NewV7()),
		OwnerID:    uuid.Must(uuid.NewV7()),
		Name:       "Flat",
		Type:       domain.PropertyTypeApartment,
		Address:    "ul. Pushkina",
		Status:     domain.PropertyStatusActive,
		Attributes: nil,
	}

	resp, err := h.propertyResponse(context.Background(), property.OwnerID, property, leasesdomain.Lease{})
	if err != nil {
		t.Fatalf("propertyResponse error: %v", err)
	}

	// The required response field must never be nil; an empty object is encoded
	// as {} rather than null.
	if resp.Attributes == nil {
		t.Fatal("resp.Attributes = nil, want empty non-nil map")
	}
	if got := len(resp.Attributes); got != 0 {
		t.Fatalf("len(resp.Attributes) = %d, want 0", got)
	}

	// Round-trip through JSON to confirm the wire shape is {} not null.
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	attrs, ok := wire["attributes"]
	if !ok {
		t.Fatal("response JSON has no \"attributes\" key")
	}
	if string(attrs) != "{}" {
		t.Fatalf("attributes wire shape = %s, want {}", string(attrs))
	}
}

func TestHandlePropertyError_AttributeValidationErrors(t *testing.T) {
	t.Parallel()

	h := newPropertyHandlersForMapping(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	err := &propertiesapp.AttributesValidationError{Errors: []domain.AttributeValidationError{
		{Field: attrFloor, Reason: "must be between -3 and 200"},
	}}
	h.handlePropertyError(rr, req, err)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}

	var prob openapi.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("decode problem: %v\nbody: %s", err, rr.Body.String())
	}
	if prob.Errors == nil {
		t.Fatal("Problem.Errors = nil, want non-nil")
	}
	if got := len(*prob.Errors); got != 1 {
		t.Fatalf("len(*Problem.Errors) = %d, want 1", got)
	}
	if got := (*prob.Errors)[0].Field; got != attrFloor {
		t.Fatalf("Problem.Errors[0].Field = %q, want \"floor\"", got)
	}
	if got := (*prob.Errors)[0].Detail; got != "must be between -3 and 200" {
		t.Fatalf("Problem.Errors[0].Detail = %q, want \"must be between -3 and 200\"", got)
	}
}

func TestProblemWithFieldErrors(t *testing.T) {
	t.Parallel()

	p := httpsupport.ProblemWithFieldErrors(
		context.Background(),
		"Bad request",
		"msg",
		[]openapi.ProblemError{{Field: attrFloor, Detail: "too high"}},
	)

	if p.Errors == nil {
		t.Fatal("Problem.Errors = nil, want non-nil")
	}
	if got := len(*p.Errors); got != 1 {
		t.Fatalf("len(*Problem.Errors) = %d, want 1", got)
	}
	if got := (*p.Errors)[0].Field; got != attrFloor {
		t.Fatalf("Problem.Errors[0].Field = %q, want \"floor\"", got)
	}
}

func TestProblemWithFieldErrors_EmptySliceOmitsErrors(t *testing.T) {
	t.Parallel()

	// Guard against an empty field-error slice being encoded as a null "errors"
	// array, which would violate the response contract.
	p := httpsupport.ProblemWithFieldErrors(
		context.Background(),
		"Bad request",
		"msg",
		nil,
	)
	if p.Errors != nil {
		t.Fatalf("Problem.Errors = %v, want nil when no field errors", p.Errors)
	}
}

func TestPropertyAttributesPtr_NilReturnsNil(t *testing.T) {
	t.Parallel()

	if got := propertyAttributesPtr(nil); got != nil {
		t.Fatalf("propertyAttributesPtr(nil) = %v, want nil", got)
	}
}

func TestPropertyAttributesPtr_NonNilReturnsMap(t *testing.T) {
	t.Parallel()

	in := &openapi.PropertyAttributes{"rooms": "2"}
	got := propertyAttributesPtr(in)
	if got == nil {
		t.Fatal("propertyAttributesPtr = nil, want non-nil pointer")
	}
	if v, ok := (*got)["rooms"]; !ok || v != "2" {
		t.Fatalf("(*propertyAttributesPtr)[rooms] = %v (ok=%v), want \"2\"", v, ok)
	}
}

func TestPropertyAttributesPtr_EmptyMapIsNotNil(t *testing.T) {
	t.Parallel()

	// An empty (but non-nil) attributes object means "clear all characteristics";
	// it must round-trip to a non-nil pointer so the service treats it as a
	// full replacement rather than an omitted field.
	in := &openapi.PropertyAttributes{}
	got := propertyAttributesPtr(in)
	if got == nil {
		t.Fatal("propertyAttributesPtr(empty map) = nil, want non-nil pointer")
	}
	if got := len(*got); got != 0 {
		t.Fatalf("len(*propertyAttributesPtr) = %d, want 0", got)
	}
}
