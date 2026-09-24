package httpsupport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// strPtr is a small helper for optional-string query parameters in tests.
func strPtr(s string) *string { return &s }

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
	propertyContact := openapi.AdminPropertyContact{Id: user.Id}
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
			"property contacts",
			openapi.AdminPropertyContactsResponse{Items: []openapi.AdminPropertyContact{propertyContact}, Total: 7},
			adminListEnvelope[openapi.AdminPropertyContact]{Items: []openapi.AdminPropertyContact{propertyContact}, Total: 7},
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
			WriteProblem(r.Context(), w, http.StatusNotFound, Problem(r.Context(), "Not found", "test"))
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

// The CSV wire-list decoders shared by the adapters: a comma split with
// TrimSpace per element and blanks dropped, so "a, b ,," is the two-element
// list. SplitCSVParam never returns nil — an absent list stays serializable
// as [] — and ParseUUIDList decodes absent or blank as nothing at all.
func TestSplitCSVParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "plain list", raw: "book,amend", want: []string{"book", "amend"}},
		{name: "spaces and blanks dropped", raw: " book , ,amend,, ", want: []string{"book", "amend"}},
		{name: "empty raw", raw: "", want: []string{}},
		{name: "blanks only", raw: " , , ", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SplitCSVParam(tt.raw)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("SplitCSVParam(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			if got == nil {
				t.Fatal("SplitCSVParam returned nil, want a non-nil empty slice")
			}
		})
	}
}

// ParseUUIDList applies the same blank-dropping rules to uuid elements; a
// malformed element aborts the decode and hands the raw element to onBad —
// the caller builds the error because each context wraps its own
// invalid-input sentinel, which the platform package cannot import.
func TestParseUUIDList(t *testing.T) {
	t.Parallel()

	t.Run("absent and blank decode to nothing", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []*string{nil, strPtr(""), strPtr("   ")} {
			got, err := ParseUUIDList(raw, func(string) error { return errors.New("unexpected bad element") })
			if err != nil {
				t.Fatalf("ParseUUIDList(%v): %v", raw, err)
			}
			if got != nil {
				t.Fatalf("ParseUUIDList(%v) = %v, want nil", raw, got)
			}
		}
	})

	t.Run("commas and blanks only decode to an empty list", func(t *testing.T) {
		t.Parallel()

		// "  ,  " trims to a non-empty ",", so the blank check does not
		// fire: the loop drops every element and the list stays empty
		// non-nil.
		got, err := ParseUUIDList(strPtr("  ,  "), func(string) error { return errors.New("unexpected bad element") })
		if err != nil {
			t.Fatalf("ParseUUIDList: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Fatalf("ParseUUIDList = %v, want a non-nil empty list", got)
		}
	})

	t.Run("trimmed elements in order, blanks dropped", func(t *testing.T) {
		t.Parallel()

		a := uuid.MustParse("0197c0a9-8b7d-7de1-a5de-6a6f1f5f0001")
		b := uuid.MustParse("0197c0a9-8b7d-7de1-a5de-6a6f1f5f0002")
		got, err := ParseUUIDList(strPtr(" "+a.String()+" , ,"+b.String()), func(string) error {
			return errors.New("unexpected bad element")
		})
		if err != nil {
			t.Fatalf("ParseUUIDList: %v", err)
		}
		if want := []uuid.UUID{a, b}; !slices.Equal(got, want) {
			t.Fatalf("ParseUUIDList = %v, want %v", got, want)
		}
	})

	t.Run("malformed element aborts with caller-built error", func(t *testing.T) {
		t.Parallel()

		sentinel := errors.New("invalid input")
		var gotBad string
		got, err := ParseUUIDList(strPtr("not-a-uuid, "), func(bad string) error {
			gotBad = bad
			return errors.Join(sentinel, errors.New(bad))
		})
		if got != nil {
			t.Fatalf("ids = %v, want nil", got)
		}
		if gotBad != "not-a-uuid" {
			t.Fatalf("bad element = %q, want the trimmed %q", gotBad, "not-a-uuid")
		}
		if !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want it to carry the caller's sentinel", err)
		}
	})
}
