package domain_test

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntityDictionaryIsTheContract pins the dictionary of eight entities
// (ADR 0062 §2): the names are the invalidation contract between the backend
// capture points and the frontend query-key families. The want side is
// literals, not the constants — comparing All() against its own constants
// would pass even if a constant's value changed — so a renamed or revalued
// entry breaks every live client silently no longer: this test fails loudly.
func TestEntityDictionaryIsTheContract(t *testing.T) {
	t.Parallel()

	want := []string{
		"payments",
		"operations",
		"tasks",
		"contacts",
		"rentals",
		"property",
		"access",
		"history",
	}
	got := make([]string, 0, len(domain.All()))
	for _, entity := range domain.All() {
		got = append(got, string(entity))
	}
	assert.ElementsMatch(t, want, got)
}

// TestAllCoversEveryConstant guards All() against duplicate entries. Go
// cannot enumerate a package's constants, so no backend test can see a new
// constant that never lands in All() — the cross-boundary dictionary check
// is held by the frontend's literal list REALTIME_ENTITY_NAMES (ADR 0062 §2),
// and this test keeps the canonical enumeration itself duplicate-free.
func TestAllCoversEveryConstant(t *testing.T) {
	t.Parallel()

	seen := make(map[domain.Entity]struct{}, len(domain.All()))
	for _, entity := range domain.All() {
		seen[entity] = struct{}{}
	}
	require.Len(t, seen, len(domain.All()), "All() repeats an entry")
}
