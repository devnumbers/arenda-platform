package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// The PUT pin wire tests (ticket #577): the strictly-decoded toggle body
// (the PUT favorite's canon #461 — a required bool cannot express its own
// absence) and the pinned_at mapping on the property response.

func TestDecodePinBody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		want    bool
		wantErr bool
	}{
		{"pin on", `{"pinned": true}`, true, false},
		{"pin off", `{"pinned": false}`, false, false},
		{"missing flag", `{}`, false, true},
		{"unknown field", `{"pinned": true, "x": 1}`, false, true},
		{"broken json", `{"pinned": `, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(tc.body))
			got, err := decodePinBody(httptest.NewRecorder(), req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got.Pinned != tc.want {
				t.Errorf("pinned = %v, want %v", got.Pinned, tc.want)
			}
		})
	}
}

func TestPropertyResponse_IncludesPinnedAt(t *testing.T) {
	t.Parallel()

	h := newPropertyHandlersForMapping(t)
	pin := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	pinned := h.propertyResponse(domain.Property{
		ID:       uuid.Must(uuid.NewV7()),
		Name:     "Pinned Flat",
		Type:     domain.PropertyTypeApartment,
		Address:  "ul. Testovaya 1",
		Status:   domain.PropertyStatusActive,
		PinnedAt: &pin,
	})
	if pinned.PinnedAt == nil || !pinned.PinnedAt.Equal(pin) {
		t.Errorf("pinned.PinnedAt = %v, want %v", pinned.PinnedAt, pin)
	}

	unpinned := h.propertyResponse(domain.Property{
		ID:      uuid.Must(uuid.NewV7()),
		Name:    "Pinned Flat",
		Type:    domain.PropertyTypeApartment,
		Address: "ul. Testovaya 1",
		Status:  domain.PropertyStatusActive,
	})
	if unpinned.PinnedAt != nil {
		t.Errorf("unpinned.PinnedAt = %v, want nil", unpinned.PinnedAt)
	}
}
