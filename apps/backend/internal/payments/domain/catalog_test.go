package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// #1069: категория «Другое» — замыкающая строка каталога (макет пикера
// 1049:34832): слаг валиден для платежа, метка резолвится в снапшот
// операции.
func TestDefaultCatalogOther(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidDefaultCategorySlug("other"), "слаг other должен быть валиден")

	entry, ok := CategoryBySlug("other")
	assert.True(t, ok, "слаг other должен резолвиться в каталоге")
	assert.Equal(t, "Другое", entry.Label)
}

// Инвариант целостности каталога: слаги уникальны — слаг каноническая
// ссылка из БД, дубликат разорвал бы резолв метки.
func TestDefaultCatalogUniqueSlugs(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, len(defaultCategories))
	for _, entry := range defaultCategories {
		assert.False(t, seen[entry.Slug], "дубликат слага %q в каталоге", entry.Slug)
		seen[entry.Slug] = true
	}
}
