package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListProperties_WireArchivedCount verifies the «Архив» gate's wire
// behavior (issue #1233): the repository's archived count travels through
// the service into the list response as the snake_case archived_count key —
// the hub hides the «Архив» button when it reads 0.
func TestListProperties_WireArchivedCount(t *testing.T) {
	t.Parallel()

	repo := &searchWireRepo{archivedCount: 2}
	h := searchWireHandler(t, repo)

	rr := httptest.NewRecorder()
	h.ListProperties(rr, searchRequest("/properties"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	count, ok := body["archived_count"]
	if !ok {
		t.Fatal("response JSON has no \"archived_count\" key")
	}
	if string(count) != "2" {
		t.Fatalf("archived_count wire value = %s, want 2", string(count))
	}
}

// TestListProperties_WireArchivedCountZero verifies the zero case: no
// archived rows — the key still rides the response (the contract's required
// field), the hub just hides the button.
func TestListProperties_WireArchivedCountZero(t *testing.T) {
	t.Parallel()

	repo := &searchWireRepo{archivedCount: 0}
	h := searchWireHandler(t, repo)

	rr := httptest.NewRecorder()
	h.ListProperties(rr, searchRequest("/properties"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	count, ok := body["archived_count"]
	if !ok {
		t.Fatal("response JSON has no \"archived_count\" key")
	}
	if string(count) != "0" {
		t.Fatalf("archived_count wire value = %s, want 0", string(count))
	}
}
