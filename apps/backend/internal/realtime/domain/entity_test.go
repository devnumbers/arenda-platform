package domain_test

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntityDictionaryIsTheContract pins the dictionary of eight entities
// (ADR 0062 §2): the names are the invalidation contract between the backend
// capture points and the frontend query-key families. A rename here breaks
// every live client silently — it must fail this test loudly instead.
func TestEntityDictionaryIsTheContract(t *testing.T) {
	t.Parallel()

	want := []domain.Entity{
		domain.EntityPayments,
		domain.EntityOperations,
		domain.EntityTasks,
		domain.EntityContacts,
		domain.EntityRentals,
		domain.EntityProperty,
		domain.EntityAccess,
		domain.EntityHistory,
	}
	assert.ElementsMatch(t, want, domain.All())

	for _, entity := range want {
		assert.NotEmpty(t, string(entity))
	}
}

// TestAllCoversEveryConstant guards the canonical enumeration: a new constant
// that never lands in All() would be invisible to the contract test above.
func TestAllCoversEveryConstant(t *testing.T) {
	t.Parallel()

	seen := make(map[domain.Entity]struct{}, len(domain.All()))
	for _, entity := range domain.All() {
		seen[entity] = struct{}{}
	}
	require.Len(t, seen, len(domain.All()), "All() repeats an entry")
}
