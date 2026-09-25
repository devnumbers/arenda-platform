//go:build integration

package http

// The realtime stream's integration family (карта #714, #716; ADR 0062):
// the whole vertical — the carrier over the real audience store (testcontainers
// PostgreSQL), the platform hub and the GET /realtime/stream handler. The
// frames ride the derived access at the moment of publication: the owner and
// the member get them, a stranger does not, and a revocation mid-stream stops
// the frames while the connection itself stays alive.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	realtimepg "github.com/nambers/arenda-planform/apps/backend/internal/realtime/adapters/postgres"
	realtimestream "github.com/nambers/arenda-planform/apps/backend/internal/realtime/adapters/stream"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedRealtimeClock struct{}

func (fixedRealtimeClock) Now() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }

var _ clock.Clock = fixedRealtimeClock{}

// fastStreamTimers shrinks the heartbeat so the liveness proof runs fast; the
// TTL stays long — the revoked member's connection must outlive the check.
func fastStreamTimers(h *RealtimeStreamHandlers) *RealtimeStreamHandlers {
	h.Heartbeat = 30 * time.Millisecond
	h.TTL = 5 * time.Second
	return h
}

// openRealtimeStream serves the handler for the user and opens the stream,
// reading up to the connected handshake.
func openRealtimeStream(t *testing.T, hub *sse.Hub, user uuid.UUID) *http.Response {
	t.Helper()
	h := fastStreamTimers(NewRealtimeStreamHandlers(hub, slog.New(slog.DiscardHandler)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.StreamRealtime(w, r.WithContext(httpsupport.WithUserID(r.Context(), user)))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	})

	handshake := readUntil(t, resp.Body, "event: connected")
	require.Contains(t, handshake, "event: connected")
	return resp
}

// readUntil reads the stream until the buffer contains the marker and returns
// everything read so far.
func readUntil(t *testing.T, body io.Reader, marker string) string {
	t.Helper()
	var acc bytes.Buffer
	buf := make([]byte, 4096)
	for !strings.Contains(acc.String(), marker) {
		n, err := body.Read(buf)
		if n > 0 {
			if _, werr := acc.Write(buf[:n]); werr != nil {
				t.Fatalf("buffer write: %v", werr)
			}
		}
		if err != nil {
			t.Fatalf("stream ended before %q: %v (read so far: %q)", marker, err, acc.String())
		}
	}
	return acc.String()
}

// drainFor reads whatever arrives within the window and returns the bytes —
// the caller asserts on their absence of the frame marker.
func drainFor(t *testing.T, body io.Reader, window time.Duration) string {
	t.Helper()
	// The response body of an httptest server stream is an io.Reader without
	// deadlines; the window is bounded by the deadline on the request context
	// the stream opened with — use a short background reader with a timeout
	// via a channel.
	type result struct {
		data string
	}
	out := make(chan result, 1)
	go func() {
		var acc bytes.Buffer
		buf := make([]byte, 4096)
		deadline := time.Now().Add(window)
		for time.Now().Before(deadline) {
			n, err := body.Read(buf)
			if n > 0 {
				if _, werr := acc.Write(buf[:n]); werr != nil {
					t.Errorf("buffer write: %v", werr)
				}
			}
			if err != nil {
				break
			}
		}
		out <- result{data: acc.String()}
	}()
	select {
	case r := <-out:
		return r.data
	case <-time.After(window + 2*time.Second):
		t.Fatalf("drain did not finish in %v", window+2*time.Second)
		return ""
	}
}

func TestRealtimeStreamAudience(t *testing.T) {
	t.Parallel()

	pool := testdb.Setup(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	outsider := uuid.Must(uuid.NewV7())
	propID := uuid.Must(uuid.NewV7())

	for _, u := range []uuid.UUID{owner, member, outsider} {
		phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
			u, phone, actor.RoleOwner)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
		 VALUES ($1, $2, $3, 'viewer', $4, 'active')`,
		uuid.Must(uuid.NewV7()), propID, member, owner)
	require.NoError(t, err)

	hub := sse.NewHub(nil)
	audience := realtimepg.NewAudienceStore(database.NewInstrumentedPool(pool, logger))
	carrier := realtimestream.NewPublisher(hub, audience, fixedRealtimeClock{}, logger)

	ownerResp := openRealtimeStream(t, hub, owner)       //nolint:bodyclose // the helper registers the body close in t.Cleanup
	memberResp := openRealtimeStream(t, hub, member)     //nolint:bodyclose // the helper registers the body close in t.Cleanup
	outsiderResp := openRealtimeStream(t, hub, outsider) //nolint:bodyclose // the helper registers the body close in t.Cleanup

	// The mutation's frame (the mutation pipeline itself is proven in the
	// contexts' integration suites): the owner and the member ride it, the
	// stranger does not.
	carrier.EntityChanged(ctx, member, domain.On(domain.EntityPayments, propID))

	ownerFrame := readUntil(t, ownerResp.Body, "event: entity.changed")
	assert.Contains(t, ownerFrame, `"propertyId":"`+propID.String()+`"`)
	assert.Contains(t, ownerFrame, `"entity":"payments"`)
	memberFrame := readUntil(t, memberResp.Body, "event: entity.changed")
	assert.Contains(t, memberFrame, propID.String())
	outsiderDrain := drainFor(t, outsiderResp.Body, 200*time.Millisecond)
	assert.NotContains(t, outsiderDrain, "event: entity.changed", "a stranger gets no frames")

	// Revocation mid-stream: the frames stop for the revoked member, the
	// connection itself stays alive (ADR 0062 §4 — no connection management).
	_, err = pool.Exec(ctx,
		`DELETE FROM property_members WHERE property_id = $1 AND user_id = $2`, propID, member)
	require.NoError(t, err)
	carrier.EntityChanged(ctx, owner, domain.On(domain.EntityOperations, propID))

	ownerFrame2 := readUntil(t, ownerResp.Body, "event: entity.changed")
	assert.Contains(t, ownerFrame2, "operations")

	revokedDrain := drainFor(t, memberResp.Body, 200*time.Millisecond)
	assert.NotContains(t, revokedDrain, "event: entity.changed",
		"the revoked member's frames stop")
	assert.Contains(t, revokedDrain, ": ping",
		"the revoked member's connection stays alive — the heartbeat flows")
}

// The archive's invisibility canon (#163, the member leg of
// actor_can_read_history in 000137) covers the realtime frames too: an
// active member of a just-archived object is no reader — the frame dies for
// them while the owner keeps reading it (the owner's leg is unconditional).
// No connection management (ADR 0062 §4): the member's stream itself stays
// alive, only the frames stop.
func TestRealtimeStreamArchivedPropertyAudience(t *testing.T) {
	t.Parallel()

	pool := testdb.Setup(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	propID := uuid.Must(uuid.NewV7())

	for _, u := range []uuid.UUID{owner, member} {
		phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
			u, phone, actor.RoleOwner)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
		 VALUES ($1, $2, $3, 'viewer', $4, 'active')`,
		uuid.Must(uuid.NewV7()), propID, member, owner)
	require.NoError(t, err)

	// The archive lands after the membership is in place — the audience
	// resolves at the moment of publication, so the archive status is what
	// must cut the member off.
	_, err = pool.Exec(ctx,
		`UPDATE properties SET status = 'archived' WHERE id = $1`, propID)
	require.NoError(t, err)

	hub := sse.NewHub(nil)
	audience := realtimepg.NewAudienceStore(database.NewInstrumentedPool(pool, logger))
	carrier := realtimestream.NewPublisher(hub, audience, fixedRealtimeClock{}, logger)

	ownerResp := openRealtimeStream(t, hub, owner)   //nolint:bodyclose // the helper registers the body close in t.Cleanup
	memberResp := openRealtimeStream(t, hub, member) //nolint:bodyclose // the helper registers the body close in t.Cleanup

	// ArchiveProperty publishes its property frame post-commit; the payments
	// pair stands in for it — the audience query is entity-agnostic.
	carrier.EntityChanged(ctx, member, domain.On(domain.EntityPayments, propID))

	ownerFrame := readUntil(t, ownerResp.Body, "event: entity.changed")
	assert.Contains(t, ownerFrame, `"propertyId":"`+propID.String()+`"`,
		"the owner reads the archive — the frame rides")
	memberDrain := drainFor(t, memberResp.Body, 200*time.Millisecond)
	assert.NotContains(t, memberDrain, "event: entity.changed",
		"the archived object's active member gets no frames")
	assert.Contains(t, memberDrain, ": ping",
		"the member's connection stays alive — the heartbeat flows")
}
