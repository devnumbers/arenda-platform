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

// Accepts is the push subscription's delivery verdict: the category flag
// gates its own category, the always-on service categories answer to nothing
// (спека #1028 — поле enabled снесено, мастер — само существование строки:
// подписки у выключенного устройства нет, проверять нечего).
func TestPushSubscription_Accepts(t *testing.T) {
	t.Parallel()

	sub := PushSubscription{Categories: DefaultCategoryPrefs()}
	assert.True(t, sub.Accepts(CategoryTasks))
	assert.True(t, sub.Accepts(CategoryTariff), "service category is always on")

	categoryOff := PushSubscription{
		Categories: CategoryPrefs{Rental: true, PaymentsOperations: true, Tasks: false, SharedAccess: true},
	}
	assert.False(t, categoryOff.Accepts(CategoryTasks))
	assert.True(t, categoryOff.Accepts(CategoryRental))
	assert.True(t, categoryOff.Accepts(CategoryTariff))
}
