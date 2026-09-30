package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// The composition security test of the paid-sections tariff gate on the
// global «Участники» section (карта #997, экран 2 #999): every /participants*
// route is re-registered over the generated OpenAPI router with
// PaidSectionsMiddleware (chi matches the last registration — the AdminOnly
// precedent). No handler re-checks the tariff, so a routing refactor that
// silently drops a re-registration would expose the section to the basic
// tariff. These tests drive the production router built by httpserver.New —
// the full middleware chain (session → tariff gate → handler) — and pin both
// directions: a basic session is refused with 402 tariff_required on every
// route of the group, and a paid session reaches the handlers.

// Session cookie values the stub loader resolves; fixed values keep the
// routing under test deterministic.
const (
	paidSectionsBasicSessionCookie = "paid-sections-basic-session-cookie"
	paidSectionsPaidSessionCookie  = "paid-sections-paid-session-cookie"
)

// paidSectionsSession is the identity one stub session resolves to.
type paidSectionsSession struct {
	userID uuid.UUID
	role   actor.Role
}

// paidSectionsLoader is the test stand-in for the identity-backed
// httpsupport.SessionLoader, mirroring the admin-gate composition test.
type paidSectionsLoader struct {
	actors map[string]paidSectionsSession
}

func (l paidSectionsLoader) Load(_ context.Context, token string, _ time.Time) (uuid.UUID, actor.Role, error) {
	a, ok := l.actors[token]
	if !ok {
		return uuid.Nil, "", httpsupport.SessionNotFound(nil)
	}
	return a.userID, a.role, nil
}

func (paidSectionsLoader) Touch(context.Context, string, string, time.Time) (string, *time.Time, error) {
	return "", nil, nil
}

// fixedPaidSectionsGate answers every request with the same verdict — the
// deny direction is the security pin, the allow one proves paid users reach
// the handlers.
type fixedPaidSectionsGate struct{ allows bool }

func (g fixedPaidSectionsGate) CanUsePaidFeatures(context.Context, uuid.UUID) (bool, error) {
	return g.allows, nil
}

// emptyParticipants is the accessapp.ParticipantsManager double: with the
// allow-direction control the only handler actually executed is the hub list,
// which returns an empty page.
type emptyParticipants struct{}

func (emptyParticipants) ListParticipants(context.Context, uuid.UUID) ([]accessapp.Participant, error) {
	return nil, nil
}

func (emptyParticipants) GetParticipant(context.Context, uuid.UUID, string) (accessapp.Participant, error) {
	return accessapp.Participant{}, nil
}

func (emptyParticipants) Summary(context.Context, uuid.UUID) (accessapp.ParticipantsSummary, error) {
	return accessapp.ParticipantsSummary{}, nil
}

// allowingSubscription is the SubscriptionMutationChecker stand-in: the
// readonly gate sits in the global chain before routing and would nil-panic
// on mutations without it — irrelevant to the tariff gate under test.
type allowingSubscription struct{}

func (allowingSubscription) CanMutateData(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

func newPaidSectionsRouter(t *testing.T, allows bool) http.Handler {
	t.Helper()
	return New(Deps{
		SessionLoader: paidSectionsLoader{actors: map[string]paidSectionsSession{
			paidSectionsBasicSessionCookie: {userID: uuid.Must(uuid.NewV7()), role: actor.RoleOwner},
			paidSectionsPaidSessionCookie:  {userID: uuid.Must(uuid.NewV7()), role: actor.RoleOwner},
		}},
		Participants:     emptyParticipants{},
		ReadonlyGate:     allowingSubscription{},
		PaidSectionsGate: fixedPaidSectionsGate{allows: allows},
		Logger:           slog.New(slog.DiscardHandler),
	})
}

func doPaidSectionsRequest(t *testing.T, router http.Handler, cookie, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, http.NoBody)
	req.AddCookie(&http.Cookie{Name: httpsupport.SessionCookieName(false), Value: cookie})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestPaidSectionsComposition_BasicRefusedOnEveryParticipantsRoute drives
// every global participants route of the production router with a basic
// tariff session and demands exactly 402 tariff_required — exceptions are not
// intended here (ADR 0064): every consumer of the group lives inside the
// gated section, «Покинуть объект» goes through /properties/{id}/access/
// members/self excluded on screen 1.
func TestPaidSectionsComposition_BasicRefusedOnEveryParticipantsRoute(t *testing.T) {
	t.Parallel()
	router := newPaidSectionsRouter(t, false)

	const participantID = "0197aaaa-bbbb-7ccc-8ddd-eeeeffff0001"
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"hub list", http.MethodGet, "/participants"},
		{"global invite", http.MethodPost, "/participants/invite"},
		{"hub summary", http.MethodGet, "/participants/summary"},
		{"participant page", http.MethodGet, "/participants/" + participantID},
		{"revoke participant", http.MethodDelete, "/participants/" + participantID},
		{"grant more properties", http.MethodPost, "/participants/" + participantID + "/properties"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := doPaidSectionsRequest(t, router, paidSectionsBasicSessionCookie, tc.method, tc.path)

			if rec.Code != http.StatusPaymentRequired {
				t.Fatalf("status = %d, want 402 on %s %s", rec.Code, tc.method, tc.path)
			}
			var problem struct {
				Code *string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem response: %v", err)
			}
			if problem.Code == nil || *problem.Code != "tariff_required" {
				t.Errorf("code = %v, want tariff_required", problem.Code)
			}
		})
	}
}

// TestPaidSectionsComposition_PaidTariffReachesHandlers proves the mount
// gates nothing away from a paid session: the request passes the tariff
// middleware and the handler answers from the service.
func TestPaidSectionsComposition_PaidTariffReachesHandlers(t *testing.T) {
	t.Parallel()
	router := newPaidSectionsRouter(t, true)

	rec := doPaidSectionsRequest(t, router, paidSectionsPaidSessionCookie, http.MethodGet, "/participants")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a paid tariff on GET /participants", rec.Code)
	}
	var response struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode participants response: %v", err)
	}
	if len(response.Items) != 0 {
		t.Errorf("items = %d, want an empty list from the stub service", len(response.Items))
	}
}
