package postgres

// The action journal's reading store (карта #704, тикет #708, ADR 0061 §7):
// the feed page, the filter-sheet participants and objects. The visibility
// predicate lives in the SQL (actor_can_read_property, 000142) — rows outside
// the reader's scope never leave the database; this adapter only marshals
// the jsonb columns back and composes the participants' display names by the
// access canon (DisplayNameOf: «Имя Фамилия», иначе «Пользователь» — карта
// #1105, аменд #1123; телефон и email в имени никогда).

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared"
)

// ReadStore reads the action journal.
type ReadStore struct {
	db postgres.DBTX
}

var _ application.EntryReader = (*ReadStore)(nil)

// NewReadStore creates a journal reading store.
func NewReadStore(db postgres.DBTX) *ReadStore {
	return &ReadStore{db: db}
}

// List returns one keyset page of the reader's journal feed, newest first.
func (s *ReadStore) List(ctx context.Context, actor uuid.UUID, q application.JournalQuery) ([]domain.FeedEntry, error) {
	// Сервис гарантирует 1..100; стор проверяет границу явно — честный
	// int→int32 без переполнения (gosec G115).
	if q.Limit <= 0 || q.Limit > application.FeedMaxLimit {
		return nil, fmt.Errorf("list action journal: limit %d out of range", q.Limit)
	}
	// After-нога (prepend новых) — отдельным ASC-запросом от якоря:
	// after-цепочка доносит всплеск новых записей целиком, без дыр.
	if q.After != nil {
		return s.listAfter(ctx, actor, q)
	}
	params := postgres.ListActionJournalParams{
		Actor:       pgconv.UUIDToPgtype(actor),
		PropertyIds: joinUUIDs(q.PropertyIDs),
		ActorIds:    joinUUIDs(q.ActorIDs),
		Kinds:       joinStrings(kindStrings(q.Kinds)),
		BaseActions: joinStrings(baseActionStrings(q.BaseActions)),
		// Поиск в двух ногах предиката: QRaw — prefix-FTS, QTrgm — ILIKE-trgm
		// (эскейп метасимволов, ESCAPE '\', — общий pgconv.EscapeLikePattern,
		// прежняя локальная «canon»-копия снесена); trim q сделал сервис
		// (sanitizeSearch).
		QRaw:  q.Search,
		QTrgm: pgconv.EscapeLikePattern(q.Search),
		// Сервис гарантирует 1..application.FeedMaxLimit (гвард List выше) —
		// сужение int→int32 ограничено этим контрактом.
		PageLimit: toPageLimit(q.Limit),
	}
	if q.DateFrom != nil {
		params.DateFrom = pgtype.Timestamptz{Time: *q.DateFrom, Valid: true}
	}
	if q.DateTo != nil {
		params.DateTo = pgtype.Timestamptz{Time: *q.DateTo, Valid: true}
	}
	if q.Before != nil {
		params.BeforeTs = pgtype.Timestamptz{Time: q.Before.CreatedAt, Valid: true}
		params.BeforeID = pgconv.UUIDToPgtype(q.Before.ID)
	}

	rows, err := postgres.New(s.db).ListActionJournal(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list action journal: %w", err)
	}

	out := make([]domain.FeedEntry, len(rows))
	for i, row := range rows {
		entry, err := feedEntryOf(row)
		if err != nil {
			return nil, err
		}
		out[i] = entry
	}
	return out, nil
}

// listAfter serves the prepend's after_cursor page: the SQL reads the window
// «строго новее якоря» from the anchor upward (created_at ASC, id ASC) — the
// page takes the OLDEST eligible rows, so a burst of new arrivals wider than
// the page is carried by the after-chain entirely, from the anchor toward
// the fresh edge (the contract's «появившиеся записи не теряются»; the DESC
// query would take the burst's newest and strand its oldest tail). The
// reversal restores the feed's DESC order, so rows[0] stays the page's
// newest row for the service's cursor math.
func (s *ReadStore) listAfter(ctx context.Context, actor uuid.UUID, q application.JournalQuery) ([]domain.FeedEntry, error) {
	params := postgres.ListActionJournalAfterParams{
		Actor:       pgconv.UUIDToPgtype(actor),
		PropertyIds: joinUUIDs(q.PropertyIDs),
		ActorIds:    joinUUIDs(q.ActorIDs),
		Kinds:       joinStrings(kindStrings(q.Kinds)),
		BaseActions: joinStrings(baseActionStrings(q.BaseActions)),
		QRaw:        q.Search,
		QTrgm:       pgconv.EscapeLikePattern(q.Search),
		AfterTs:     pgtype.Timestamptz{Time: q.After.CreatedAt, Valid: true},
		AfterID:     pgconv.UUIDToPgtype(q.After.ID),
		// Сервис гарантирует 1..application.FeedMaxLimit (гвард List выше) —
		// сужение int→int32 ограничено этим контрактом.
		PageLimit: toPageLimit(q.Limit),
	}
	if q.DateFrom != nil {
		params.DateFrom = pgtype.Timestamptz{Time: *q.DateFrom, Valid: true}
	}
	if q.DateTo != nil {
		params.DateTo = pgtype.Timestamptz{Time: *q.DateTo, Valid: true}
	}

	rows, err := postgres.New(s.db).ListActionJournalAfter(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list action journal after: %w", err)
	}
	slices.Reverse(rows)

	out := make([]domain.FeedEntry, len(rows))
	for i, row := range rows {
		// Оба запроса читают одни и те же колонки — конвертация рядов
		// безпотерьна, маппинг общий.
		entry, err := feedEntryOf(postgres.ListActionJournalRow(row))
		if err != nil {
			return nil, err
		}
		out[i] = entry
	}
	return out, nil
}

// FilterParticipants returns the scope's participants: the owner and the
// current members ∪ every historical actor of the scope's journal (the
// UNION legs live in the SQL). The display name is composed live by the
// access canon — the chip is UI metadata, the row snapshots stay in the
// journal.
func (s *ReadStore) FilterParticipants(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) ([]domain.FilterParticipant, error) {
	rows, err := postgres.New(s.db).ListHistoryFilterParticipants(ctx, postgres.ListHistoryFilterParticipantsParams{
		Actor:       pgconv.UUIDToPgtype(actor),
		PropertyIds: joinUUIDs(propertyIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("list history filter participants: %w", err)
	}
	out := make([]domain.FilterParticipant, len(rows))
	for i, row := range rows {
		out[i] = domain.FilterParticipant{
			ID: pgconv.UUIDFromPgtype(row.ID),
			Name: accessapp.DisplayNameOf(accessapp.MemberUser{
				ID:      pgconv.UUIDFromPgtype(row.ID),
				Name:    pgconv.TextToPtrString(row.Name),
				Surname: pgconv.TextToPtrString(row.Surname),
				Phone:   row.Phone,
			}),
			Email:     pgconv.TextToString(row.Email),
			FirstName: row.FirstName,
			PhotoURL:  row.PhotoPath,
			IsOwner:   row.IsOwner,
			Role:      row.Role,
		}
	}
	return out, nil
}

// FilterObjects returns the scope's objects with the card photo avatar —
// the first (oldest) photo or ” without any.
func (s *ReadStore) FilterObjects(ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID) ([]domain.FilterObject, error) {
	rows, err := postgres.New(s.db).ListHistoryFilterObjects(ctx, postgres.ListHistoryFilterObjectsParams{
		Actor:       pgconv.UUIDToPgtype(actor),
		PropertyIds: joinUUIDs(propertyIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("list history filter objects: %w", err)
	}
	out := make([]domain.FilterObject, len(rows))
	for i, row := range rows {
		out[i] = domain.FilterObject{
			ID:       pgconv.UUIDFromPgtype(row.ID),
			Name:     row.Name,
			Address:  row.Address,
			PhotoURL: row.PhotoUrl,
			Type:     row.Type,
		}
	}
	return out, nil
}

// feedEntryOf unmarshals the row's jsonb columns back into the read
// projection. A malformed segments/context blob is a data-integrity failure,
// not a wire error — it surfaces as a 500 through the wrapped error.
func feedEntryOf(row postgres.ListActionJournalRow) (domain.FeedEntry, error) {
	var segments domain.Segments
	if err := json.Unmarshal(row.Segments, &segments); err != nil {
		return domain.FeedEntry{}, fmt.Errorf("unmarshal history segments %s: %w", pgconv.UUIDFromPgtype(row.ID), err)
	}
	var contextMap map[string]any
	if err := json.Unmarshal(row.Context, &contextMap); err != nil {
		return domain.FeedEntry{}, fmt.Errorf("unmarshal history context %s: %w", pgconv.UUIDFromPgtype(row.ID), err)
	}
	var actorID *uuid.UUID
	if row.ActorID.Valid {
		id := pgconv.UUIDFromPgtype(row.ActorID)
		actorID = &id
	}
	return domain.FeedEntry{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		PropertyName:  row.PropertyName,
		ActorID:       actorID,
		ActorName:     row.ActorName,
		ActorEmail:    row.ActorEmail,
		ActorPhotoURL: row.ActorPhotoUrl,
		ActorRole:     domain.ActorRole(row.ActorRole),
		Kind:          domain.Kind(row.Kind),
		Action:        domain.Action(row.Action),
		BaseAction:    domain.BaseAction(row.BaseAction),
		Segments:      segments,
		Context:       contextMap,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}

func kindStrings(kinds []domain.Kind) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

func baseActionStrings(actions []domain.BaseAction) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}

func joinStrings(parts []string) string {
	return strings.Join(parts, ",")
}

// joinUUIDs renders the property/actor ids as the CSV string the sqlc
// queries cast through string_to_array → uuid[].
func joinUUIDs(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}

// toPageLimit сужает page-limit фида до int32 SQL-параметра: сервис
// гарантирует 1..application.FeedMaxLimit (гвард List выше), сечение —
// санкционированный shared.ToInt32Clamped (clamp к границам int32).
func toPageLimit(limit int) int32 {
	return shared.ToInt32Clamped(limit)
}
