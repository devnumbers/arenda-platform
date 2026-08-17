package tkassa

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa/spec"
)

func TestSign(t *testing.T) {
	data := map[string]any{
		"TerminalKey": "TinkoffBankTest",
		"Amount":      int64(1000),
		"OrderId":     "order-123",
		"DATA": map[string]string{
			"OperationInitiatorType": string(spec.CommonOperationInitiatorTypeN1),
		},
		"Receipt": map[string]any{
			"Items": []any{"item1"},
		},
		"Description": nil,
	}

	got := sign(data, testPassword)

	// Build expected concatenation manually in sorted key order:
	// Amount, OrderId, Password, TerminalKey.
	wantConcat := "1000" + "order-123" + testPassword + "TinkoffBankTest"
	hash := sha256.Sum256([]byte(wantConcat))
	want := hex.EncodeToString(hash[:])
	if got != want {
		t.Fatalf("sign mismatch: got %q, want %q", got, want)
	}
}

func TestSignSkipsNullBlankAndNested(t *testing.T) {
	data := map[string]any{
		"TerminalKey": "Term",
		"Amount":      int64(1),
		"OrderId":     "o",
		"NullValue":   nil,
		"Blank":       "",
		"DATA":        map[string]string{"k": "v"},
		"Receipt":     []any{1, 2},
	}

	got := sign(data, testPassword)
	// Sorted keys: Amount, Blank, NullValue, OrderId, Password, TerminalKey.
	// NullValue (nil), Blank (empty string), DATA and Receipt are all skipped.
	wantConcat := "1" + "o" + testPassword + "Term"
	hash := sha256.Sum256([]byte(wantConcat))
	want := hex.EncodeToString(hash[:])
	if got != want {
		t.Fatalf("sign mismatch: got %q, want %q", got, want)
	}
}

// TestSignFixedVector pins the token algorithm to a fixed test vector so a
// regression in sorting, concatenation or hashing is caught byte-exactly.
func TestSignFixedVector(t *testing.T) {
	data := map[string]any{
		"TerminalKey": testTerminalKey,
		"OrderId":     "11111111-1111-1111-1111-111111111111",
		"Amount":      json.Number("10000"),
		"Success":     true,
	}
	got := sign(data, testPassword)

	// Sorted keys: Amount, OrderId, Password, Success, TerminalKey.
	wantConcat := "10000" + "11111111-1111-1111-1111-111111111111" + testPassword + "true" + testTerminalKey
	hash := sha256.Sum256([]byte(wantConcat))
	want := hex.EncodeToString(hash[:])
	if got != want {
		t.Fatalf("sign fixed vector: got %q, want %q", got, want)
	}
}

func TestSignDoesNotMutateInput(t *testing.T) {
	data := map[string]any{
		"TerminalKey": "Term",
		"Amount":      json.Number("1"),
	}
	before := mustMarshal(t, data)
	_ = sign(data, testPassword)
	after := mustMarshal(t, data)
	if before != after {
		t.Fatalf("sign mutated input: before %s, after %s", before, after)
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
