// Package postgres implements the contacts application ports with SQLC
// queries over PostgreSQL (ADR 0051).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var (
	_ application.ContactStore  = (*ContactStore)(nil)
	_ application.PropertyStore = (*PropertyStore)(nil)
)

// ContactStore is the postgres adapter of the contact book port (ADR 0051).
// Reads and writes are scoped by the data owner; the by-id read is unscoped
// by contract — the service authorizes from the card's own binding.
type ContactStore struct {
	db postgres.DBTX
}

// NewContactStore creates a contact store over the given connection or pool.
func NewContactStore(db postgres.DBTX) *ContactStore {
	return &ContactStore{db: db}
}

func (s *ContactStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *ContactStore) WithTx(tx transaction.Tx) (application.ContactStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("contacts.ContactStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewContactStore(dbtx), nil
}

// GetByID loads one card; pgx.ErrNoRows — an unknown id — becomes the
// application ErrNotFound.
func (s *ContactStore) GetByID(ctx context.Context, id uuid.UUID) (domain.Contact, error) {
	row, err := s.q().GetContactByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Contact{}, application.ErrNotFound
		}
		return domain.Contact{}, fmt.Errorf("get contact %s: %w", id, err)
	}
	return contactFromRow(row), nil
}

// List returns the actor's visible contacts per the query scope, search and
// sort (” search = no filter, ” sort/order = the defaults), each with the
// display name of its bound property.
func (s *ContactStore) List(
	ctx context.Context, actorID uuid.UUID, q application.ListQuery,
) ([]application.ListedContact, error) {
	rows, err := s.q().ListContacts(ctx, postgres.ListContactsParams{
		ActorID:    pgconv.UUIDToPgtype(actorID),
		Scope:      string(q.Scope),
		PropertyID: pgconv.UUIDToPgtype(q.PropertyID),
		Search:     escapeLikePattern(q.Search),
		Sort:       string(q.Sort),
		Order:      string(q.Order),
	})
	if err != nil {
		return nil, fmt.Errorf("list contacts of actor %s: %w", actorID, err)
	}
	contacts := make([]application.ListedContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, application.ListedContact{
			Contact:      contactFromListRow(row),
			PropertyName: pgconv.TextToString(row.PropertyName),
		})
	}
	return contacts, nil
}

// Create inserts a new card and returns the stored row with its timestamps.
func (s *ContactStore) Create(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	row, err := s.q().InsertContact(ctx, postgres.InsertContactParams{
		ID:                pgconv.UUIDToPgtype(c.ID),
		OwnerID:           pgconv.UUIDToPgtype(c.OwnerID),
		PropertyID:        pgconv.UUIDToPgtypePtr(c.PropertyID),
		FirstName:         c.FirstName,
		LastName:          textOrNull(c.LastName),
		Patronymic:        textOrNull(c.Patronymic),
		Role:              textOrNull(c.Role),
		Phone:             textOrNull(c.Phone),
		Email:             textOrNull(c.Email),
		MessengerName:     textOrNull(c.MessengerName),
		MessengerUsername: textOrNull(c.MessengerUsername),
		Note:              textOrNull(c.Note),
	})
	if err != nil {
		return domain.Contact{}, fmt.Errorf("insert contact %s: %w", c.ID, err)
	}
	return contactFromRow(row), nil
}

// Update rewrites the editable fields keyed by (id, owner_id); pgx.ErrNoRows
// — the card gone between the service's read and this write — is the
// application ErrNotFound.
func (s *ContactStore) Update(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	row, err := s.q().UpdateContact(ctx, postgres.UpdateContactParams{
		ID:                pgconv.UUIDToPgtype(c.ID),
		OwnerID:           pgconv.UUIDToPgtype(c.OwnerID),
		PropertyID:        pgconv.UUIDToPgtypePtr(c.PropertyID),
		FirstName:         c.FirstName,
		LastName:          textOrNull(c.LastName),
		Patronymic:        textOrNull(c.Patronymic),
		Role:              textOrNull(c.Role),
		Phone:             textOrNull(c.Phone),
		Email:             textOrNull(c.Email),
		MessengerName:     textOrNull(c.MessengerName),
		MessengerUsername: textOrNull(c.MessengerUsername),
		Note:              textOrNull(c.Note),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Contact{}, application.ErrNotFound
		}
		return domain.Contact{}, fmt.Errorf("update contact %s: %w", c.ID, err)
	}
	return contactFromRow(row), nil
}

// Delete removes the card keyed by (id, owner_id); zero rows deleted is
// ErrNotFound — the card is gone between the service's read and this write.
func (s *ContactStore) Delete(ctx context.Context, id, ownerID uuid.UUID) error {
	deleted, err := s.q().DeleteContact(ctx, postgres.DeleteContactParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		return fmt.Errorf("delete contact %s: %w", id, err)
	}
	if deleted == 0 {
		return application.ErrNotFound
	}
	return nil
}

// PropertyStore is the postgres adapter of the property reference port: just
// the owner whose book the property-bound cards land in.
type PropertyStore struct {
	db postgres.DBTX
}

// NewPropertyStore creates a property store over the given connection or pool.
func NewPropertyStore(db postgres.DBTX) *PropertyStore {
	return &PropertyStore{db: db}
}

func (s *PropertyStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// Get loads the property reference; pgx.ErrNoRows — an unknown id — becomes
// the application ErrNotFound.
func (s *PropertyStore) Get(ctx context.Context, propertyID uuid.UUID) (application.PropertyRef, error) {
	row, err := s.q().GetContactPropertyRef(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PropertyRef{}, application.ErrNotFound
		}
		return application.PropertyRef{}, fmt.Errorf("get property ref %s: %w", propertyID, err)
	}
	return application.PropertyRef{OwnerID: pgconv.UUIDFromPgtype(row.OwnerID)}, nil
}

// contactFromRow maps the generated row onto the domain card: NULL folds to
// "" (the domain's «not set»).
func contactFromRow(row postgres.Contact) domain.Contact {
	return domain.Contact{
		ID:                pgconv.UUIDFromPgtype(row.ID),
		OwnerID:           pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:        pgconv.UUIDFromPgtypePtr(row.PropertyID),
		FirstName:         row.FirstName,
		LastName:          pgconv.TextToString(row.LastName),
		Patronymic:        pgconv.TextToString(row.Patronymic),
		Role:              pgconv.TextToString(row.Role),
		Phone:             pgconv.TextToString(row.Phone),
		Email:             pgconv.TextToString(row.Email),
		MessengerName:     pgconv.TextToString(row.MessengerName),
		MessengerUsername: pgconv.TextToString(row.MessengerUsername),
		Note:              pgconv.TextToString(row.Note),
		CreatedAt:         pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:         pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}

// contactFromListRow maps the listing row (the card's columns plus the
// bound property's display name from the LEFT JOIN) onto the domain card.
func contactFromListRow(row postgres.ListContactsRow) domain.Contact {
	return domain.Contact{
		ID:                pgconv.UUIDFromPgtype(row.ID),
		OwnerID:           pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:        pgconv.UUIDFromPgtypePtr(row.PropertyID),
		FirstName:         row.FirstName,
		LastName:          pgconv.TextToString(row.LastName),
		Patronymic:        pgconv.TextToString(row.Patronymic),
		Role:              pgconv.TextToString(row.Role),
		Phone:             pgconv.TextToString(row.Phone),
		Email:             pgconv.TextToString(row.Email),
		MessengerName:     pgconv.TextToString(row.MessengerName),
		MessengerUsername: pgconv.TextToString(row.MessengerUsername),
		Note:              pgconv.TextToString(row.Note),
		CreatedAt:         pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:         pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}

// textOrNull folds the domain's «not set» ("" — never written by the
// service's normalization) onto SQL NULL.
func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// likePatternEscaper escapes the ILIKE metacharacters in user-supplied search
// text. The matching SQL pattern uses ESCAPE '\'.
var likePatternEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLikePattern trims and escapes a user-supplied substring so it can be
// safely embedded in an ILIKE '%...%' pattern. An empty result disables the
// filter on the SQL side.
func escapeLikePattern(q string) string {
	return likePatternEscaper.Replace(strings.TrimSpace(q))
}
