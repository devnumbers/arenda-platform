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
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The composition security test of the paid-sections tariff gate on the
// gated sections (карта #997; ADR 0064): the participants group's routes are
// re-registered over the generated OpenAPI router with PaidSectionsMiddleware
// (chi matches the last registration — the AdminOnly precedent). No handler
// of that group re-checks the tariff, so a routing refactor that silently
// drops a re-registration would expose the section to the basic tariff.
// These tests drive the production router built by httpserver.New — the full
// middleware chain (session → tariff gate → handler). The other two gated
// surfaces left the middleware mount: the per-object access group in ревизия
// #1462 and the history journal in ревизия #1465 — their role-aware gates
// live in the use cases after the already-resolved role, so here the tests
// pin the opposite direction too: a basic session's narrowed membership read
// of the journal passes the production router, and the un-narrowed one is
// refused by the handler itself with 402 tariff_required. The use-case
// verdicts are pinned by the access and history contexts' own tests, the 402
// wire shapes by the handlers' tests.

// Session cookie values the stub loader resolves; fixed values keep the
// routing under test deterministic. The case names of the journal endpoints
// are shared by the three history tests (goconst): the refusal, the narrowed
// pass and the paid direction all drive the same two routes.
const (
	paidSectionsBasicSessionCookie = "paid-sections-basic-session-cookie"
	paidSectionsPaidSessionCookie  = "paid-sections-paid-session-cookie"

	caseJournalFeed   = "journal feed"
	caseFilterOptions = "filter options"
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

// emptyHistory is the historyapp.EntryReader double: the history handlers
// answer from it with empty pages.
type emptyHistory struct{}

func (emptyHistory) List(context.Context, uuid.UUID, historyapp.JournalQuery) ([]historydomain.FeedEntry, error) {
	return nil, nil
}

func (emptyHistory) FilterParticipants(context.Context, uuid.UUID, []uuid.UUID) ([]historydomain.FilterParticipant, error) {
	return nil, nil
}

func (emptyHistory) FilterObjects(context.Context, uuid.UUID, []uuid.UUID) ([]historydomain.FilterObject, error) {
	return nil, nil
}

// memberEverywhere is the sharedpolicy.Policy double of the composition
// tests: every property resolves to an active viewer membership — the
// privacy pass opens the narrowed scope, the tariff verdict stays with the
// gate. Role resolution itself is the service unit tests' territory.
type memberEverywhere struct{}

func (memberEverywhere) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleViewer, nil
}

func (memberEverywhere) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleViewer, nil
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
	gate := fixedPaidSectionsGate{allows: allows}
	return New(Deps{
		SessionLoader: paidSectionsLoader{actors: map[string]paidSectionsSession{
			paidSectionsBasicSessionCookie: {userID: uuid.Must(uuid.NewV7()), role: actor.RoleOwner},
			paidSectionsPaidSessionCookie:  {userID: uuid.Must(uuid.NewV7()), role: actor.RoleOwner},
		}},
		Participants: emptyParticipants{},
		// The journal's role-aware tariff gate (ревизия #1465) consumes the
		// same fixed gate as the middleware: the composition pins that the
		// handler-owned verdict reaches the wire — and that the narrowed
		// membership read passes on the basic tariff.
		HistoryRead:      historyapp.NewHistoryReadService(emptyHistory{}, memberEverywhere{}, gate, nil),
		ReadonlyGate:     allowingSubscription{},
		PaidSectionsGate: gate,
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

// TestPaidSectionsComposition_BasicRefusedOnEveryGatedRoute drives every
// route of the composition-covered group — the global participants section —
// of the production router with a basic tariff session and demands exactly
// 402 tariff_required. Exceptions are not intended in the group (ADR 0064):
// every consumer of the participants group lives inside the gated section.
// The per-object access group is not composition-covered since ревизия
// #1462, the history journal since ревизия #1465: their gates are role-aware
// and live in the use cases after the already-resolved role — the routing
// refactor that once dropped a re-registration there can no longer expose
// the sections, the refusals are owned by the handlers themselves (the
// application tests pin the verdicts, the handler tests pin the 402
// tariff_required wire shape, the history composition tests below pin the
// verdict reaching the wire through this router).
func TestPaidSectionsComposition_BasicRefusedOnEveryGatedRoute(t *testing.T) {
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

// TestPaidSectionsComposition_BasicHistoryRefusalsLiveInTheHandlers pins the
// handler-owned refusal of the journal (ревизия #1465): with the middleware
// mount gone, an un-narrowed basic read — the global feed's owner scope by
// construction — is still answered with exactly 402 tariff_required, now by
// HistoryReadService's verdict through the handlers.
func TestPaidSectionsComposition_BasicHistoryRefusalsLiveInTheHandlers(t *testing.T) {
	t.Parallel()
	router := newPaidSectionsRouter(t, false)

	cases := []struct {
		name string
		path string
	}{
		{caseJournalFeed, "/history"},
		{caseFilterOptions, "/history/filters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := doPaidSectionsRequest(t, router, paidSectionsBasicSessionCookie, http.MethodGet, tc.path)

			if rec.Code != http.StatusPaymentRequired {
				t.Fatalf("status = %d, want 402 on GET %s", rec.Code, tc.path)
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

// TestPaidSectionsComposition_BasicMemberNarrowedHistoryPasses pins «
// пригласили — значит пользуется» on the production router (ревизия #1465):
// a basic session reading the journal narrowed to a membership property
// passes — the direction a middleware re-mount would silently break.
func TestPaidSectionsComposition_BasicMemberNarrowedHistoryPasses(t *testing.T) {
	t.Parallel()
	router := newPaidSectionsRouter(t, false)

	propertyID := uuid.Must(uuid.NewV7()).String()
	cases := []struct {
		name string
		path string
	}{
		{caseJournalFeed, "/history?property_ids=" + propertyID},
		{caseFilterOptions, "/history/filters?property_ids=" + propertyID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := doPaidSectionsRequest(t, router, paidSectionsBasicSessionCookie, http.MethodGet, tc.path)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 for the narrowed membership read on GET %s", rec.Code, tc.path)
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

// TestPaidSectionsComposition_PaidTariffReachesHistoryHandlers proves the
// same for the «История действий» journal: both read endpoints of the
// section answer 200 from the service on a paid tariff.
func TestPaidSectionsComposition_PaidTariffReachesHistoryHandlers(t *testing.T) {
	t.Parallel()
	router := newPaidSectionsRouter(t, true)

	cases := []struct {
		name string
		path string
	}{
		{caseJournalFeed, "/history"},
		{caseFilterOptions, "/history/filters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := doPaidSectionsRequest(t, router, paidSectionsPaidSessionCookie, http.MethodGet, tc.path)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 for a paid tariff on GET %s", rec.Code, tc.path)
			}
		})
	}
}
