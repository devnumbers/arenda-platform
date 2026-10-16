package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The category × channel matrix (решение #738, ADR 0058): four configurable
// categories per set, the service categories never gated.
func TestCategoryPrefs_DefaultIsAllOn(t *testing.T) {
	t.Parallel()

	prefs := DefaultCategoryPrefs()
	assert.True(t, prefs.Rental)
	assert.True(t, prefs.PaymentsOperations)
	assert.True(t, prefs.Tasks)
	assert.True(t, prefs.SharedAccess)
}

func TestCategoryPrefs_Allows(t *testing.T) {
	t.Parallel()

	off := CategoryPrefs{Rental: true, PaymentsOperations: false, Tasks: true, SharedAccess: false}

	assert.True(t, off.Allows(CategoryRental))
	assert.False(t, off.Allows(CategoryPaymentsOperations))
	assert.True(t, off.Allows(CategoryTasks))
	assert.False(t, off.Allows(CategorySharedAccess))

	// The service categories are outside the settings screen — nothing can
	// turn them off (решение чарта, ADR 0058).
	assert.True(t, DefaultCategoryPrefs().Allows(CategoryTariff))
	assert.True(t, DefaultCategoryPrefs().Allows(CategorySystem))
	assert.True(t, off.Allows(CategoryTariff))
	assert.True(t, off.Allows(CategorySystem))
}
