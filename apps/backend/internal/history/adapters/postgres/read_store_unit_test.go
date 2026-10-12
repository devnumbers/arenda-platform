package postgres

// The unit seam of the reading store's row mapping (решение #1317 §9):
// feedEntryOf — NULL-детали легаси остаются nil, пустые живые части актёра
// из COALESCE маппятся как есть, битый details-блоб — ошибка данных (канон
// segments/context). SQL-поведение (COALESCE, JOIN) покрывает интеграционный
// сюит; здесь — только конвертация.

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

func feedEntryRow(details []byte, first, last string) postgres.ListActionJournalRow {
	return postgres.ListActionJournalRow{
		ID:             pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		PropertyID:     pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		PropertyName:   "Квартира",
		ActorID:        pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		ActorName:      "Иван Иванов",
		ActorEmail:     "ivan@example.com",
		ActorFirstName: first,
		ActorLastName:  last,
		ActorRole:      "owner",
		ActorPhotoUrl:  "",
		Kind:           "property",
		Action:         "property.renamed",
		BaseAction:     "changed",
		Segments: []byte(`[{"text":"Название объекта изменено: "},` +
			`{"text":"Новое","link":{"kind":"property","id":"00000000-0000-0000-0000-000000000002"}}]`),
		Details:   details,
		Context:   []byte(`{}`),
		CreatedAt: pgtype.Timestamptz{Valid: true},
	}
}

func TestFeedEntryOf_LegacyNilDetailsAndEmptyLiveParts(t *testing.T) {
	t.Parallel()
	// Легаси: details NULL → nil (не пустой список); пустые части из
	// COALESCE проходят как есть — сборку имени делает фронт.
	entry, err := feedEntryOf(feedEntryRow(nil, "", ""))
	if err != nil {
		t.Fatalf("feedEntryOf: %v", err)
	}
	if entry.Details != nil {
		t.Errorf("details = %q, want nil", entry.Details)
	}
	if entry.ActorFirstName != "" || entry.ActorLastName != "" {
		t.Errorf("live parts = %q/%q, want empty as mapped", entry.ActorFirstName, entry.ActorLastName)
	}
}

func TestFeedEntryOf_DetailsRoundTrip(t *testing.T) {
	t.Parallel()
	entry, err := feedEntryOf(feedEntryRow([]byte(`["Название: «Old» → «Новое»"]`), "Иван", "Иванов"))
	if err != nil {
		t.Fatalf("feedEntryOf: %v", err)
	}
	if len(entry.Details) != 1 || entry.Details[0] != "Название: «Old» → «Новое»" {
		t.Errorf("details = %q, want the single line", entry.Details)
	}
	if entry.ActorFirstName != "Иван" || entry.ActorLastName != "Иванов" {
		t.Errorf("live parts = %q/%q, want as mapped", entry.ActorFirstName, entry.ActorLastName)
	}
}

func TestFeedEntryOf_MalformedDetailsIsDataError(t *testing.T) {
	t.Parallel()
	_, err := feedEntryOf(feedEntryRow([]byte(`{broken`), "", ""))
	if err == nil || !strings.Contains(err.Error(), "unmarshal history details") {
		t.Fatalf("err = %v, want the details data-integrity error", err)
	}
}
