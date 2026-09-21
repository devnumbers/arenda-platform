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

// Accepts is the push subscription's delivery verdict: the master toggle
// gates everything, the category flag gates its own category.
func TestPushSubscription_Accepts(t *testing.T) {
	t.Parallel()

	on := PushSubscription{Enabled: true, Categories: DefaultCategoryPrefs()}
	assert.True(t, on.Accepts(CategoryTasks))
	assert.True(t, on.Accepts(CategoryTariff), "service category answers to the master only")

	categoryOff := PushSubscription{
		Enabled:    true,
		Categories: CategoryPrefs{Rental: true, PaymentsOperations: true, Tasks: false, SharedAccess: true},
	}
	assert.False(t, categoryOff.Accepts(CategoryTasks))
	assert.True(t, categoryOff.Accepts(CategoryRental))
	assert.True(t, categoryOff.Accepts(CategoryTariff))

	// Master-off mutes every category, service ones included (2333-180696).
	masterOff := PushSubscription{Enabled: false, Categories: DefaultCategoryPrefs()}
	assert.False(t, masterOff.Accepts(CategoryRental))
	assert.False(t, masterOff.Accepts(CategoryTariff))
}
