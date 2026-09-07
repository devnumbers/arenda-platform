package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewMaterializedTask snapshots the rule's content onto the task (resolution
// #496); the property binding is part of that snapshot — property-bound and
// property-less rules alike (ADR 0052).
func TestNewMaterializedTaskSnapshots(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())
	rule := uuid.Must(uuid.NewV7())
	comment := "проверить"
	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

	t.Run("property-bound rule", func(t *testing.T) {
		t.Parallel()
		task := NewMaterializedTask(TaskRule{
			ID:         rule,
			OwnerID:    owner,
			PropertyID: &property,
			Title:      "Поменять лампочку",
			Comment:    &comment,
			DueDate:    &due,
			Repeat:     RepeatWeekly,
		}, &due)

		assert.Equal(t, owner, task.OwnerID)
		require.NotNil(t, task.PropertyID)
		assert.Equal(t, property, *task.PropertyID)
		require.NotNil(t, task.RuleID)
		assert.Equal(t, rule, *task.RuleID)
		assert.Equal(t, "Поменять лампочку", task.Title)
		require.NotNil(t, task.Comment)
		assert.Equal(t, comment, *task.Comment)
		assert.Equal(t, due.UTC(), task.DueDate.UTC())
	})

	t.Run("property-less rule", func(t *testing.T) {
		t.Parallel()
		task := NewMaterializedTask(TaskRule{
			ID:      rule,
			OwnerID: owner,
			Title:   "Позвонить бухгалтеру",
			Repeat:  RepeatOnce,
		}, nil)

		assert.Nil(t, task.PropertyID)
		assert.Nil(t, task.DueDate)
		assert.Equal(t, "Позвонить бухгалтеру", task.Title)
	})
}
