package httpsupport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// The generic list envelope behind RespondAdminList must serialize exactly
// like the per-entity oapi-codegen response structs, so the helper can stand
// in for the generated types without changing the wire format. The empty-page
// case pins the no-omitempty contract: items render as [] and total as 0.
func TestRespondAdminListMatchesGeneratedEnvelope(t *testing.T) {
	t.Parallel()

	type userView struct{ phone string }
	mapper := func(v userView) openapi.AdminUser { return openapi.AdminUser{Phone: v.phone} }
	noErrorHandler := func(http.ResponseWriter, *http.Request, error) {
		t.Error("unexpected handleError call")
	}

	tests := []struct {
		name  string
		views []userView
		total int64
		want  openapi.AdminUsersResponse
	}{
		{
			name:  "items with total",
			views: []userView{{phone: "+79001112233"}, {phone: "+79004445566"}},
			total: 42,
			want: openapi.AdminUsersResponse{
				Items: []openapi.AdminUser{{Phone: "+79001112233"}, {Phone: "+79004445566"}},
				Total: 42,
			},
		},
		{
			name:  "empty page keeps empty items and zero total",
			views: []userView{},
			total: 0,
			want: openapi.AdminUsersResponse{
				Items: []openapi.AdminUser{},
				Total: 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			RespondAdminList(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/users", nil),
				func(context.Context) ([]userView, int64, error) { return tt.views, tt.total, nil },
				noErrorHandler,
				mapper,
			)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			want, err := json.Marshal(tt.want)
			if err != nil {
				t.Fatalf("marshal generated envelope: %v", err)
			}
			// WriteJSON encodes with json.Encoder, which appends a newline.
			if got := strings.TrimSuffix(rec.Body.String(), "\n"); got != string(want) {
				t.Fatalf("body = %s, want %s", got, want)
			}
		})
	}
}

// TestAdminListEnvelopeTagsPinGeneratedResponses guards the JSON tags of the
// generic envelope against drift in every generated {items, total} response
// the admin API actually returns via RespondAdminList.
func TestAdminListEnvelopeTagsPinGeneratedResponses(t *testing.T) {
	t.Parallel()

	user := openapi.AdminUser{Phone: "+79001112233"}
	property := openapi.AdminProperty{Id: user.Id}
	lease := openapi.AdminLease{Id: user.Id}
	tenantContact := openapi.AdminTenantContact{Id: user.Id}
	propertyContact := openapi.AdminPropertyContact{Id: user.Id}
	operation := openapi.AdminOperation{Id: user.Id}
	auditLog := openapi.AdminAuditLog{Id: user.Id}

	tests := []struct {
		name      string
		generated any
		envelope  any
	}{
		{
			"users",
			openapi.AdminUsersResponse{Items: []openapi.AdminUser{user}, Total: 7},
			adminListEnvelope[openapi.AdminUser]{Items: []openapi.AdminUser{user}, Total: 7},
		},
		{
			"properties",
			openapi.AdminPropertiesResponse{Items: []openapi.AdminProperty{property}, Total: 7},
			adminListEnvelope[openapi.AdminProperty]{Items: []openapi.AdminProperty{property}, Total: 7},
		},
		{
			"leases",
			openapi.AdminLeasesResponse{Items: []openapi.AdminLease{lease}, Total: 7},
			adminListEnvelope[openapi.AdminLease]{Items: []openapi.AdminLease{lease}, Total: 7},
		},
		{
			"tenant contacts",
			openapi.AdminTenantContactsResponse{Items: []openapi.AdminTenantContact{tenantContact}, Total: 7},
			adminListEnvelope[openapi.AdminTenantContact]{Items: []openapi.AdminTenantContact{tenantContact}, Total: 7},
		},
		{
			"property contacts",
			openapi.AdminPropertyContactsResponse{Items: []openapi.AdminPropertyContact{propertyContact}, Total: 7},
			adminListEnvelope[openapi.AdminPropertyContact]{Items: []openapi.AdminPropertyContact{propertyContact}, Total: 7},
		},
		{
			"operations",
			openapi.AdminOperationsResponse{Items: []openapi.AdminOperation{operation}, Total: 7},
			adminListEnvelope[openapi.AdminOperation]{Items: []openapi.AdminOperation{operation}, Total: 7},
		},
		{
			"audit logs",
			openapi.AdminAuditLogsResponse{Items: []openapi.AdminAuditLog{auditLog}, Total: 7},
			adminListEnvelope[openapi.AdminAuditLog]{Items: []openapi.AdminAuditLog{auditLog}, Total: 7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			generated, err := json.Marshal(tt.generated)
			if err != nil {
				t.Fatalf("marshal generated envelope: %v", err)
			}
			envelope, err := json.Marshal(tt.envelope)
			if err != nil {
				t.Fatalf("marshal generic envelope: %v", err)
			}
			if string(generated) != string(envelope) {
				t.Fatalf("generated = %s, generic envelope = %s", generated, envelope)
			}
		})
	}
}

func TestRespondAdminListDelegatesErrors(t *testing.T) {
	t.Parallel()

	type view struct{}
	rec := httptest.NewRecorder()
	problemWritten := false
	RespondAdminList(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/users", nil),
		func(context.Context) ([]view, int64, error) { return nil, 0, errors.New("boom") },
		func(w http.ResponseWriter, r *http.Request, _ error) {
			problemWritten = true
			WriteProblem(w, http.StatusNotFound, Problem(r.Context(), "Not found", "test"))
		},
		func(view) openapi.AdminUser { return openapi.AdminUser{} },
	)

	if !problemWritten {
		t.Fatal("handleError was not called")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
