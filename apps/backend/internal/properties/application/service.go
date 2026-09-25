// Package application holds the properties use cases and ports: property lifecycle (create, archive, delete),
// photos and contacts.
package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	realtimedom "github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const (
	maxPhotoCount = 10
	// MaxPhotoSize caps a single property photo upload; the HTTP adapter
	// enforces the same bound while buffering a streamed multipart upload.
	MaxPhotoSize   = 5 * 1024 * 1024 // 5 MiB.
	photoKeyPrefix = "properties"
)

var allowedPhotoContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type CreatePropertyCommand struct {
	Name        string
	Type        string
	Address     string
	Description string
	Attributes  map[string]any
}

type UpdatePropertyCommand struct {
	Name        *string
	Type        *string
	Address     *string
	Description *string
	Attributes  *map[string]any
	Status      *string
}

type PropertyService struct {
	txStoreFactory
	repo               PropertyRepository
	photoRepo          PropertyPhotoRepository
	photoStorage       PhotoStorage
	audit              auditapp.Recorder
	clock              clock.Clock
	policy             sharedpolicy.Policy
	sharedMemberships  SharedMemberships
	ownerNames         OwnerDisplayNameResolver
	ownerEmails        OwnerEmailResolver
	suspendedShared    SuspendedSharedMemberships
	slots              RecipientSlotPolicy
	sharedDeleteMailer SharedMembersDeleteMailer
	rentalOccupancy    RentalOccupancyReader
	overdueOperations  OverdueOperationsReader
	rentalDeletion     RentalDeletionGuard
	ownerCalendar      OwnerCalendar
	logger             *slog.Logger
	// Realtime is the late-bound carrier the mutations' frames dispatch
	// through after the commit (карта #714, #716; ADR 0062); nil keeps the
	// pre-#716 silence.
	realtime realtimeapp.Publisher
}

// SetRealtimePublisher late-binds the realtime carrier (карта #714, #716;
// ADR 0062): the frames of the committed mutations dispatch through it —
// the grace-events canon, best-effort, a broken carrier never fails the
// mutation.
func (s *PropertyService) SetRealtimePublisher(p realtimeapp.Publisher) {
	s.realtime = p
}

// publishChanged hands the committed object mutation's frames to the realtime
// carrier (карта #714, #716; ADR 0062) — strictly post-commit, best-effort: a
// rolled-back transaction dispatches nothing, a nil carrier keeps the
// pre-#716 silence. A journaled mutation piggybacks the history pair — a
// written row is a history change for the object's feed. (The delete's frame
// has no audience by construction: post-commit the object row is gone and the
// derived access resolves to nobody, so DeleteProperty dispatches nothing.)
func (s *PropertyService) publishChanged(ctx context.Context, actor, propertyID uuid.UUID, journaled bool) {
	if s.realtime == nil {
		return
	}
	changed := []realtimedom.Change{realtimedom.On(realtimedom.EntityProperty, propertyID)}
	if journaled {
		changed = append(changed, realtimedom.HistoryOn(propertyID))
	}
	s.realtime.EntityChanged(ctx, actor, changed...)
}

// SetSharedMemberships injects the access-context adapter that resolves the
// active shared-access memberships of a user (property id + recipient role) so
// list endpoints include properties shared with the actor and expose the
// actor's access role (issues #156 T3, T11). Optional: when not set, only the
// owner's own properties are listed.
func (s *PropertyService) SetSharedMemberships(memberships SharedMemberships) {
	s.sharedMemberships = memberships
}

// SetOwnerDisplayNameResolver injects the access-context adapter that resolves
// the public display name of a property owner for the sharing banner (issue
// T11). Optional: when not set, detail responses carry no owner name.
func (s *PropertyService) SetOwnerDisplayNameResolver(resolver OwnerDisplayNameResolver) {
	s.ownerNames = resolver
}

// SetOwnerEmailResolver injects the adapter that resolves a property owner's
// account email for the detail's owner contact row (Figma 2200-97365) —
// a deliberate exposure on this surface, same posture as
// suspended_shared.owner_email (#702). Optional: when not set, detail
// responses carry no owner email.
func (s *PropertyService) SetOwnerEmailResolver(resolver OwnerEmailResolver) {
	s.ownerEmails = resolver
}

// SetSuspendedSharedMemberships injects the access-context adapter that
// resolves the recipient's suspended shared memberships — the blur-card
// placeholders of the main list (ticket #702, replacing the hidden-shared
// count of issues #158 T4 and #163). Optional: when not set, the list
// carries no placeholders.
func (s *PropertyService) SetSuspendedSharedMemberships(port SuspendedSharedMemberships) {
	s.suspendedShared = port
}

// SetRecipientSlotPolicy injects the access-context slot coordinator that
// enforces the recipient tariff slot invariant for shared-access memberships of
// a property (issue #158, T4). Optional: when not set, archiving/unarchiving/
// deleting a property does not suspend or recover shared memberships (the
// pre-T4 behaviour).
func (s *PropertyService) SetRecipientSlotPolicy(slots RecipientSlotPolicy) {
	s.slots = slots
}

// SetSharedMembersDeleteMailer injects the access-context mailer that notifies
// former shared members when the owner deletes a shared object (issue #162,
// T6). Optional: when not set, deleting a property sends no such emails.
func (s *PropertyService) SetSharedMembersDeleteMailer(mailer SharedMembersDeleteMailer) {
	s.sharedDeleteMailer = mailer
}

// SetRentalOccupancyReader injects the rentals-context adapter that resolves
// the per-property occupancy («Занятость», резолюция #584) for the list
// reads. Optional: when not set, the lists report no occupancy (ticket #585).
func (s *PropertyService) SetRentalOccupancyReader(reader RentalOccupancyReader) {
	s.rentalOccupancy = reader
}

// SetRentalDeletionGuard injects the rentals-context adapter that gates the
// property deletion on the rentals state (issue #632): an unfinished rental
// conflicts (ErrPropertyOccupied), completed ones are torn down before the
// property row. Optional: when not set, deletion skips both steps (the
// pre-#632 behaviour).
func (s *PropertyService) SetRentalDeletionGuard(guard RentalDeletionGuard) {
	s.rentalDeletion = guard
}

// SetOverdueOperationsReader injects the payments-context adapter that
// resolves which properties have overdue planned operations (the payments
// half of the red dot, резолюция #584). Optional: when not set, the lists
// report no overdue flags (ticket #585).
func (s *PropertyService) SetOverdueOperationsReader(reader OverdueOperationsReader) {
	s.overdueOperations = reader
}

// SetOwnerCalendar injects the calendar adapter that resolves the reading
// actor's calendar date (ADR 0048) for the list responses (ticket #586).
// Optional: when not set, the lists fall back to the server clock's UTC date.
func (s *PropertyService) SetOwnerCalendar(calendar OwnerCalendar) {
	s.ownerCalendar = calendar
}

// enrichListProjections fills the per-property occupancy and the overdue flag
// over the merged list (own + shared rows alike; ticket #585): one batched
// read per projection, the owners resolved from the rows themselves. The
// readers are optional: an unwired projection stays unreported. Every listed
// property gets an occupancy — the reader reports OccupancyNone for a
// property without an unfinished rental.
func (s *PropertyService) enrichListProjections(ctx context.Context, properties []domain.Property) error {
	if len(properties) == 0 {
		return nil
	}
	owners := make(PropertyOwners, len(properties))
	for i := range properties {
		owners[properties[i].ID] = properties[i].OwnerID
	}
	if s.rentalOccupancy != nil {
		occupancy, err := s.rentalOccupancy.OccupancyByProperty(ctx, owners)
		if err != nil {
			return fmt.Errorf("read property occupancy: %w", err)
		}
		for i := range properties {
			if o, ok := occupancy[properties[i].ID]; ok {
				properties[i].Occupancy = &o
				continue
			}
			properties[i].Occupancy = &domain.Occupancy{Status: domain.OccupancyNone}
		}
	}
	if s.overdueOperations != nil {
		overdue, err := s.overdueOperations.OverdueByProperty(ctx, owners)
		if err != nil {
			return fmt.Errorf("read property overdue operations: %w", err)
		}
		for i := range properties {
			properties[i].HasOverdueOperations = overdue[properties[i].ID]
		}
	}
	return nil
}

// NewPropertyService creates a PropertyService. Persistence, the audit
// recorder and the Unit-of-Work of every mutating use case arrive through the
// embedded factory (ADR 0033 γ-factory); repo and photoRepo additionally
// serve the non-transactional reads and the externally-owned transaction of
// ArchiveExcessProperties.
func NewPropertyService(
	repo PropertyRepository,
	photoRepo PropertyPhotoRepository,
	photoStorage PhotoStorage,
	factory txStoreFactory,
	clk clock.Clock,
	policy sharedpolicy.Policy,
	logger *slog.Logger,
) *PropertyService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PropertyService{
		txStoreFactory: factory,
		repo:           repo,
		photoRepo:      photoRepo,
		photoStorage:   photoStorage,
		audit:          factory.audit,
		clock:          clk,
		policy:         policy,
		logger:         logger,
	}
}

func (s *PropertyService) CreateProperty(ctx context.Context, actor uuid.UUID, cmd CreatePropertyCommand) (domain.Property, error) {
	propertyType, err := domain.ParsePropertyType(cmd.Type)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
	}

	attrs := domain.Attributes(cmd.Attributes)
	if result := domain.ValidateAttributes(propertyType, attrs); !result.Valid() {
		return domain.Property{}, &AttributesValidationError{Errors: result.Errors}
	}

	property, err := domain.NewProperty(actor, cmd.Name, cmd.Address, cmd.Description, propertyType, attrs)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	now := s.clock.Now()
	property.CreatedAt = now
	property.UpdatedAt = now

	var created domain.Property
	err = s.runInTx(ctx, func(stores *txStores) error {
		limit, err := stores.limiter.ActivePropertyLimit(ctx, actor)
		if err != nil {
			return fmt.Errorf("get active property limit: %w", err)
		}

		count, err := stores.repo.CountActiveByOwner(ctx, actor)
		if err != nil {
			return fmt.Errorf("count active properties: %w", err)
		}
		if count >= limit {
			return ErrLimitExceeded
		}

		created, err = stores.repo.Create(ctx, actor, property)
		if err != nil {
			return fmt.Errorf("create property: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyCreated,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &created.ID,
			Context:    map[string]any{"name": created.Name, "type": string(created.Type)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, created.ID, actor,
			sharedpolicy.HistoryActorRole(sharedpolicy.RoleOwner),
			historydomain.PropertyCreated(created.ID, created.Name)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, created.ID, true)

	created.Photos = []domain.Photo{}
	created.AccessRole = sharedpolicy.RoleOwner
	return created, nil
}

func (s *PropertyService) ListProperties(ctx context.Context, actor uuid.UUID) (PropertiesPage, error) {
	today, err := s.listToday(ctx, actor)
	if err != nil {
		return PropertiesPage{}, err
	}

	properties, err := s.repo.ListActiveByOwner(ctx, actor)
	if err != nil {
		return PropertiesPage{}, fmt.Errorf("list properties: %w", err)
	}

	// Own properties are read with the owner role (issue T11).
	for i := range properties {
		properties[i].AccessRole = sharedpolicy.RoleOwner
	}

	// Only active and maintenance properties belong in the main list.
	properties, err = s.appendSharedProperties(ctx, actor, properties,
		domain.PropertyStatusActive, domain.PropertyStatusMaintenance)
	if err != nil {
		return PropertiesPage{}, err
	}

	s.enrichSharedOwnerNames(ctx, properties)

	// The actor's suspended shared memberships ride the main list as
	// blur-card placeholders (ticket #702); the archived list carries none.
	suspended, err := s.listSuspendedShared(ctx, actor)
	if err != nil {
		return PropertiesPage{}, err
	}

	if err := s.enrichListProjections(ctx, properties); err != nil {
		return PropertiesPage{}, err
	}

	items, err := s.withPhotos(ctx, s.pinnedFirst(properties)...)
	if err != nil {
		return PropertiesPage{}, err
	}
	return PropertiesPage{Items: items, Today: today, SuspendedShared: suspended}, nil
}

// enrichSharedOwnerNames fills the owner display name of the shared list
// rows (owner decision on the #756 walkthrough fixes): the shared card
// shows whose object it is; own rows keep none — the reader is the owner.
// The resolver is optional and failure-degrading, the detail-read
// precedent: an unwired port or a failed lookup leaves the row without
// the name.
func (s *PropertyService) enrichSharedOwnerNames(ctx context.Context, properties []domain.Property) {
	if s.ownerNames == nil {
		return
	}
	resolved := make(map[uuid.UUID]string)
	for i := range properties {
		if properties[i].AccessRole == sharedpolicy.RoleOwner {
			continue
		}
		ownerID := properties[i].OwnerID
		name, ok := resolved[ownerID]
		if !ok {
			var err error
			name, err = s.ownerNames.DisplayName(ctx, ownerID)
			if err != nil {
				s.logger.WarnContext(ctx, "failed to resolve owner display name",
					slog.String("owner_id", ownerID.String()),
					slog.String("error", sanitizeError(err)),
				)
			}
			resolved[ownerID] = name
		}
		properties[i].OwnerName = name
	}
}

// listSuspendedShared resolves the actor's suspended shared memberships for
// the main list (ticket #702). The port is optional: an unwired read reports
// no placeholders.
func (s *PropertyService) listSuspendedShared(ctx context.Context, actor uuid.UUID) ([]SharedSuspendedMembership, error) {
	if s.suspendedShared == nil {
		return nil, nil
	}
	suspended, err := s.suspendedShared.SuspendedWith(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("read suspended shared: %w", err)
	}
	return suspended, nil
}

// pinnedFirst lifts the pinned properties above the unpinned ones (ticket
// #577): among themselves by the pin time — the first pin stays on top, a
// re-pin never shifts the order; the unpinned keep the incoming order
// (the repository's updated_at DESC, the shared properties at their end).
// Stable so equal pins never swap. The SQL list already orders by the pin
// (ListActivePropertiesByOwner); this re-sort extends the same rule over
// the merged list, whose shared rows were appended after the SQL pass.
func (s *PropertyService) pinnedFirst(properties []domain.Property) []domain.Property {
	slices.SortStableFunc(properties, func(a, b domain.Property) int {
		switch {
		case a.PinnedAt == nil && b.PinnedAt != nil:
			return 1
		case a.PinnedAt != nil && b.PinnedAt == nil:
			return -1
		case a.PinnedAt != nil && b.PinnedAt != nil && a.PinnedAt.Before(*b.PinnedAt):
			return -1
		case a.PinnedAt != nil && b.PinnedAt != nil && b.PinnedAt.Before(*a.PinnedAt):
			return 1
		}
		return 0
	})
	return properties
}

// appendSharedProperties appends the properties shared with the actor to the
// actor's own list (issues #156 T3, T11). Each shared property belongs to
// another owner: it is loaded by id, deduplicated against the own set in case
// of an accidental self-membership, and kept only when its status is one of
// the wanted list statuses.
func (s *PropertyService) appendSharedProperties(
	ctx context.Context, actor uuid.UUID, properties []domain.Property, wanted ...domain.PropertyStatus,
) ([]domain.Property, error) {
	if s.sharedMemberships == nil {
		return properties, nil
	}
	memberships, err := s.sharedMemberships.MembershipsWith(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list shared memberships: %w", err)
	}
	seen := make(map[uuid.UUID]bool, len(properties))
	for i := range properties {
		seen[properties[i].ID] = true
	}
	for _, m := range memberships {
		if !isSharedAccessRole(m.Role) {
			// Defensive filter for corrupt data: an unrecognized membership
			// role degrades to RoleNone in the access adapter and must not
			// leak into the API contract as access.role "none"; such a
			// membership is unusable anyway, so skip it and log a warning.
			s.logger.WarnContext(ctx, "skipping shared membership with unrecognized role",
				slog.String("property_id", m.PropertyID.String()),
				slog.String("user_id", actor.String()),
			)
			continue
		}
		if seen[m.PropertyID] {
			continue
		}
		p, err := s.repo.GetByID(ctx, m.PropertyID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("load shared property: %w", err)
		}
		if !slices.Contains(wanted, p.Status) {
			continue
		}
		p.AccessRole = m.Role
		seen[m.PropertyID] = true
		properties = append(properties, p)
	}
	return properties, nil
}

// isSharedAccessRole reports whether a membership role is one of the two
// recognizable shared-access roles; anything else is corrupt data.
func isSharedAccessRole(role sharedpolicy.Role) bool {
	return role == sharedpolicy.RoleFullAccess || role == sharedpolicy.RoleViewer
}

func (s *PropertyService) ListArchivedProperties(ctx context.Context, actor uuid.UUID) (PropertiesPage, error) {
	today, err := s.listToday(ctx, actor)
	if err != nil {
		return PropertiesPage{}, err
	}

	properties, err := s.repo.ListArchivedByOwner(ctx, actor)
	if err != nil {
		return PropertiesPage{}, fmt.Errorf("list archived properties: %w", err)
	}

	// Own properties are read with the owner role (issue T11).
	for i := range properties {
		properties[i].AccessRole = sharedpolicy.RoleOwner
	}

	// The archive list carries archived objects only.
	properties, err = s.appendSharedProperties(ctx, actor, properties, domain.PropertyStatusArchived)
	if err != nil {
		return PropertiesPage{}, err
	}

	s.enrichSharedOwnerNames(ctx, properties)

	if err := s.enrichListProjections(ctx, properties); err != nil {
		return PropertiesPage{}, err
	}

	items, err := s.withPhotos(ctx, properties...)
	if err != nil {
		return PropertiesPage{}, err
	}
	return PropertiesPage{Items: items, Today: today}, nil
}

// listToday resolves the reading actor's calendar date (ADR 0048) for the
// list responses (ticket #586). The calendar port is optional: unwired (unit
// tests), the server clock's UTC date stands in.
func (s *PropertyService) listToday(ctx context.Context, actor uuid.UUID) (time.Time, error) {
	if s.ownerCalendar != nil {
		today, err := s.ownerCalendar.Today(ctx, actor)
		if err != nil {
			return time.Time{}, fmt.Errorf("resolve actor today: %w", err)
		}
		return today, nil
	}
	now := s.clock.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
}

const (
	// DefaultPropertySearchPageSize is the search page the contract serves
	// when the client names no limit (ticket #601) — the other searches'
	// canon (#597).
	DefaultPropertySearchPageSize = 50
	MaxPropertySearchPageSize     = 100
	// MaxSearchQueryLength is the contract's search-parameter bound.
	maxSearchQueryLength = 255
)

// SearchPropertiesPage is the search list's page request (ticket #601): the
// 50-per-page window the search screen's infinite scroll walks. The zero
// limit means the contract's default page; Cursor is the previous page's
// opaque continuation ("" = from the beginning).
type SearchPropertiesPage struct {
	Limit  int32
	Cursor string
}

// SearchProperties narrows the actor's visible slice by the search query —
// a case-insensitive substring over the property's name and address — and
// returns the matched cards in pages of 50, the search screen's infinite
// scroll walking them (ticket #601). The window is the keyset walk over
// (name, id) (ticket #597's pattern): the page resumes strictly after the
// cursor's key and answers with the next page's cursor once it came back
// full. The name sort mirrors the hub's own default (the client sorts the
// list by name), so the search results read in the same order the hub
// shows; a rename shifting a row across a window boundary is inherent to
// the visible-name sort, as in the contacts book (#600).
func (s *PropertyService) SearchProperties(
	ctx context.Context, actor uuid.UUID, search string, page SearchPropertiesPage,
) ([]domain.Property, string, error) {
	if trimmed := strings.TrimSpace(search); trimmed == "" {
		return nil, "", fmt.Errorf("%w: empty search", ErrInvalidInput)
	} else if utf8.RuneCountInString(trimmed) > maxSearchQueryLength {
		// The contract's maxLength counts characters (codepoints), not bytes —
		// a 130-letter Cyrillic query is legal and must not 400.
		return nil, "", fmt.Errorf("%w: search exceeds %d characters", ErrInvalidInput, maxSearchQueryLength)
	}
	if page.Limit == 0 {
		page.Limit = DefaultPropertySearchPageSize
	}
	if page.Limit < 1 || page.Limit > MaxPropertySearchPageSize {
		return nil, "", fmt.Errorf("%w: limit out of range", ErrInvalidInput)
	}
	query := PropertySearchQuery{Search: search, Limit: page.Limit}
	if page.Cursor != "" {
		afterName, afterID, err := DecodePropertySearchCursor(page.Cursor)
		if err != nil {
			return nil, "", err
		}
		query.AfterName = &afterName
		query.AfterID = &afterID
	}
	properties, err := s.repo.SearchVisible(ctx, actor, query)
	if err != nil {
		return nil, "", fmt.Errorf("search visible properties: %w", err)
	}
	properties, err = s.withPhotos(ctx, properties...)
	if err != nil {
		return nil, "", err
	}
	// A full page answers with the last row's continuation; a short one has
	// walked the matches to the end — the empty cursor stops the scroll.
	nextCursor := ""
	if len(properties) == int(page.Limit) {
		last := properties[len(properties)-1]
		nextCursor = EncodePropertySearchCursor(last.Name, last.ID)
	}
	return properties, nextCursor, nil
}

func (s *PropertyService) GetProperty(ctx context.Context, actor, id uuid.UUID) (domain.Property, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanView(role) {
		// A suspended membership is the single exception to the privacy rule:
		// the recipient gets a distinguishable signal so the UI can show the
		// "tariff limit exceeded" screen (T9, issue #158).
		if role == sharedpolicy.RoleSuspended {
			return domain.Property{}, ErrAccessSuspended
		}
		// Privacy: a missing property and lack of access both look like 404 so
		// the existence of an object is never revealed (issue #156, T3).
		return domain.Property{}, ErrNotFound
	}
	property, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	// The actor's access context for the sharing banner (issue T11): the role
	// is always reported; the owner's public display name is resolved only for
	// recipients, and a resolver failure degrades to an empty name.
	property.AccessRole = role
	if role != sharedpolicy.RoleOwner && s.ownerNames != nil {
		name, err := s.ownerNames.DisplayName(ctx, property.OwnerID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to resolve owner display name",
				slog.String("property_id", property.ID.String()),
				slog.String("error", sanitizeError(err)),
			)
		} else {
			property.OwnerName = name
		}
	}
	// The owner's account email for the detail's owner contact row (Figma
	// 2200-97365): recipients only, degrading to empty like the name.
	if role != sharedpolicy.RoleOwner && s.ownerEmails != nil {
		email, err := s.ownerEmails.GetEmail(ctx, property.OwnerID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to resolve owner email",
				slog.String("property_id", property.ID.String()),
				slog.String("error", sanitizeError(err)),
			)
		} else {
			property.OwnerEmail = email
		}
	}

	properties, err := s.withPhotos(ctx, property)
	if err != nil {
		return domain.Property{}, err
	}
	return properties[0], nil
}

func (s *PropertyService) UpdateProperty(ctx context.Context, actor, id uuid.UUID, cmd UpdatePropertyCommand) (domain.Property, error) {
	role, err := s.resolveEditableProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, err
	}

	var updated domain.Property
	var journaled bool
	err = s.runInTx(ctx, func(stores *txStores) error {
		property, err := lockEditableProperty(ctx, stores.repo, id)
		if err != nil {
			return err
		}

		before := property
		if err := applyPropertyUpdate(ctx, stores, &property, cmd); err != nil {
			return err
		}

		property.UpdatedAt = s.clock.Now()

		updated, err = stores.repo.Update(ctx, property.OwnerID, property)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("update property: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyUpdated,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &id,
			Context:    map[string]any{auditFieldsKey: updatedPropertyFields(cmd)},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if entry, ok := propertyUpdateHistoryEntry(id, before, updated); ok {
			if err := historyapp.RecordScoped(ctx, stores.history, id, actor,
				sharedpolicy.HistoryActorRole(role), entry); err != nil {
				return err
			}
			journaled = true
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, id, journaled)

	properties, err := s.withPhotos(ctx, updated)
	if err != nil {
		return domain.Property{}, err
	}
	properties[0].AccessRole = role
	return properties[0], nil
}

// SetPropertyPin writes the global pin atomically (PUT pin, ticket #577, the
// PUT favorite's canon #461): one UPDATE inside the transaction under the
// edit capability — Owner and Full Access (ADR 0028 CanEdit), a viewer is
// ErrForbidden, no access is ErrNotFound, an archived property is
// ErrArchivedProperty. A re-pin keeps the original pin time (the PUT's
// idempotency — the order among the pinned never shifts); unpinning clears
// it. The pin is a pure read-order flag: no re-read is asked for — the row
// resolved under the property lock travels out through the response.
func (s *PropertyService) SetPropertyPin(ctx context.Context, actor, id uuid.UUID, pinned bool) (domain.Property, error) {
	role, err := s.resolveEditableProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, err
	}

	var updated domain.Property
	var journaled bool
	err = s.runInTx(ctx, func(stores *txStores) error {
		property, err := lockEditableProperty(ctx, stores.repo, id)
		if err != nil {
			return err
		}

		pinnedAt := resolvedPinnedAt(property.PinnedAt, pinned, s.clock.Now())

		// The journal records a state change, not an idempotent re-pin: a
		// PUT that keeps the pin (or the absence) writes no history row.
		pinChanged := pinned != (property.PinnedAt != nil)

		updated, err = stores.repo.SetPin(ctx, id, property.OwnerID, pinnedAt)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("set property pin: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyUpdated,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &id,
			Context:    map[string]any{auditFieldsKey: []string{"pinnedAt"}},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if pinChanged {
			entry := historydomain.PropertyPinned(id)
			if !pinned {
				entry = historydomain.PropertyUnpinned(id)
			}
			if err := historyapp.RecordScoped(ctx, stores.history, id, actor,
				sharedpolicy.HistoryActorRole(role), entry); err != nil {
				return err
			}
			journaled = true
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, id, journaled)

	properties, err := s.withPhotos(ctx, updated)
	if err != nil {
		return domain.Property{}, err
	}
	properties[0].AccessRole = role
	return properties[0], nil
}

// resolveEditableProperty authorizes a property write (UpdateProperty,
// AddPropertyPhoto): the actor needs the edit capability on the object. A
// missing property or no access maps to ErrNotFound so the existence of an
// object is never revealed (issue #156, T3); a view-only role is ErrForbidden.
func (s *PropertyService) resolveEditableProperty(ctx context.Context, actor, id uuid.UUID) (sharedpolicy.Role, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone {
		return "", ErrNotFound
	}
	if !sharedpolicy.CanEdit(role) {
		return "", ErrForbidden
	}
	return role, nil
}

// lockEditableProperty loads the property row for update inside a write
// transaction and rejects archived objects: an archived property is read-only
// history.
func lockEditableProperty(ctx context.Context, repo PropertyRepository, id uuid.UUID) (domain.Property, error) {
	property, err := repo.GetByIDForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}
	if property.Status == domain.PropertyStatusArchived {
		return domain.Property{}, ErrArchivedProperty
	}
	return property, nil
}

// applyPropertyUpdate applies the command's diff to the property and
// re-validates the aggregate; only the provided fields change.
func applyPropertyUpdate(ctx context.Context, stores *txStores, property *domain.Property, cmd UpdatePropertyCommand) error {
	if cmd.Name != nil {
		property.Name = *cmd.Name
	}
	if cmd.Address != nil {
		property.Address = *cmd.Address
	}
	if cmd.Description != nil {
		property.Description = *cmd.Description
	}
	if cmd.Type != nil {
		propertyType, err := domain.ParsePropertyType(*cmd.Type)
		if err != nil {
			return fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
		}
		property.Type = propertyType
	}
	if cmd.Status != nil {
		if err := applyStatusUpdate(ctx, stores, property, *cmd.Status); err != nil {
			return err
		}
	}
	if cmd.Attributes != nil {
		attrs := domain.Attributes(*cmd.Attributes)
		if result := domain.ValidateAttributes(property.Type, attrs); !result.Valid() {
			return &AttributesValidationError{Errors: result.Errors}
		}
		property.Attributes = attrs
	}

	if err := property.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	return nil
}

// applyStatusUpdate parses and applies a requested status transition.
func applyStatusUpdate(_ context.Context, _ *txStores, property *domain.Property, rawStatus string) error {
	status, err := domain.ParsePropertyStatus(rawStatus)
	if err != nil {
		return fmt.Errorf("%w: invalid property status: %w", ErrInvalidInput, err)
	}
	if !isUpdatableStatusTransition(property.Status, status) {
		return &InvalidStatusTransitionError{From: property.Status, To: status}
	}
	property.Status = status
	return nil
}

func (s *PropertyService) ArchiveProperty(ctx context.Context, actor, id uuid.UUID) (domain.Property, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone {
		return domain.Property{}, ErrNotFound
	}
	if !sharedpolicy.CanLifecycle(role) {
		return domain.Property{}, ErrForbidden
	}

	var archived domain.Property
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		archived, err = s.archivePropertyInTx(ctx, stores.repo, actor, id)
		if err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyArchived,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &id,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, id, actor,
			sharedpolicy.HistoryActorRole(role), historydomain.PropertyArchived(id)); err != nil {
			return err
		}

		// Archiving a shared object freed one tariff slot for each recipient: try to
		// recover their oldest suspended memberships FIFO in the same transaction
		// (issue #158, T4).
		if s.slots != nil {
			if err := s.slots.RecoverSuspendedForProperty(ctx, stores.tx, id); err != nil {
				return fmt.Errorf("recover suspended memberships after archive: %w", err)
			}
			// Archiving one of the owner's OWN objects also freed one of the owner's
			// own tariff slots: the owner is never a member row of their own object,
			// so RecoverSuspendedForProperty above did not visit them. Recover their
			// own suspended shared queue FIFO in the same transaction.
			if err := s.slots.RecoverSuspended(ctx, stores.tx, actor); err != nil {
				return fmt.Errorf("recover owner suspended memberships after archive: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, id, true)

	properties, err := s.withPhotos(ctx, archived)
	if err != nil {
		return domain.Property{}, err
	}
	properties[0].AccessRole = role
	return properties[0], nil
}

// DeleteProperty removes the property together with all its data: the FKs
// cascade rentals, payments, operations, tasks and photos off the property
// row, contacts unbind (ADR 0054). ADR 0049 closed the ADR 0025 detach
// branch — deletion is total, there is no mode (issue #629).
func (s *PropertyService) DeleteProperty(
	ctx context.Context,
	actor, id uuid.UUID,
) error {
	if _, err := s.resolveLifecycleProperty(ctx, actor, id); err != nil {
		return err
	}

	// The deleted property's name, photos and former members' emails escape
	// the work closure for the post-commit cleanup.
	var (
		property           domain.Property
		photos             []domain.Photo
		formerMemberEmails []string
	)
	err := s.runInTx(ctx, func(stores *txStores) error {
		var err error
		property, err = lockDeletableProperty(ctx, stores, actor, id)
		if err != nil {
			return err
		}

		formerMemberEmails, err = s.collectFormerMemberEmails(ctx, stores, id)
		if err != nil {
			return err
		}

		// Deletion guard (issue #632): the check and the rentals teardown
		// run under the property row lock — CreateRental locks the same row
		// (ADR 0025 §5), so a rental cannot slip in between the check and
		// the delete. DeleteByProperty goes before the property row: the
		// rentals.payment_id RESTRICT FK must not race the payments cascade
		// off the property row, completed rentals included — every rental
		// row restricts its managed payment (ADR 0025 §2 explicit order).
		if s.rentalDeletion != nil {
			occupied, err := s.rentalDeletion.HasUnfinished(ctx, stores.tx, property.OwnerID, id)
			if err != nil {
				return fmt.Errorf("check unfinished rental: %w", err)
			}
			if occupied {
				return ErrPropertyOccupied
			}
			if err := s.rentalDeletion.DeleteByProperty(ctx, stores.tx, property.OwnerID, id); err != nil {
				return fmt.Errorf("delete property rentals: %w", err)
			}
		}

		photos, err = stores.photos.GetByPropertyID(ctx, id)
		if err != nil {
			return fmt.Errorf("list photos: %w", err)
		}

		if err := s.recoverSlotsAfterDelete(ctx, stores, actor, id); err != nil {
			return err
		}

		if err := stores.repo.Delete(ctx, id, actor); err != nil {
			return fmt.Errorf("delete property: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyDeleted,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &id,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Post-commit cleanup never fails the delete itself.
	s.cleanupPropertyPhotos(ctx, id, photos)
	s.notifyPropertyDeleted(ctx, id, property.Name, formerMemberEmails)

	return nil
}

// resolveLifecycleProperty authorizes a property lifecycle change
// (ArchiveProperty, UnarchiveProperty, DeleteProperty): only the owner may
// perform one (CanLifecycle). A missing property or no access maps to
// ErrNotFound so the existence of an object is never revealed (issue #156, T3).
func (s *PropertyService) resolveLifecycleProperty(ctx context.Context, actor, id uuid.UUID) (sharedpolicy.Role, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone {
		return "", ErrNotFound
	}
	if !sharedpolicy.CanLifecycle(role) {
		return "", ErrForbidden
	}
	return role, nil
}

// lockDeletableProperty loads the property row for update inside the delete
// transaction.
func lockDeletableProperty(ctx context.Context, stores *txStores, owner, id uuid.UUID) (domain.Property, error) {
	property, err := stores.repo.GetByIDAndOwnerForUpdate(ctx, id, owner)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}
	return property, nil
}

// collectFormerMemberEmails gathers the emails of the property's shared
// members before the slot policy drops their memberships, so the "object
// deleted" email can reach them after the commit (issue #162, T6).
func (s *PropertyService) collectFormerMemberEmails(ctx context.Context, stores *txStores, id uuid.UUID) ([]string, error) {
	if s.sharedDeleteMailer == nil {
		return nil, nil
	}
	emails, err := s.sharedDeleteMailer.CollectFormerMemberEmails(ctx, stores.tx, id)
	if err != nil {
		return nil, fmt.Errorf("collect former shared members: %w", err)
	}
	return emails, nil
}

// notifyPropertyDeleted emails the former shared members (active+suspended)
// after the delete has committed (issue #162, T6). A send failure is logged
// and does not affect the delete.
func (s *PropertyService) notifyPropertyDeleted(ctx context.Context, id uuid.UUID, name string, formerMemberEmails []string) {
	if s.sharedDeleteMailer == nil {
		return
	}
	for _, to := range formerMemberEmails {
		if err := s.sharedDeleteMailer.SendPropertyDeleted(ctx, to, name); err != nil {
			s.logger.ErrorContext(ctx, "failed to send property deleted email to former member",
				slog.String("property_id", id.String()),
				slog.String("error", sanitizeError(err)),
			)
		}
	}
}

// recoverSlotsAfterDelete frees the recipients' tariff slots dropped by the
// delete: the access context drops their memberships and recovers the oldest
// suspended ones FIFO in the same transaction, before the property row is
// removed (issue #158, T4). Deleting one of the owner's OWN objects also freed
// one of the owner's own tariff slots: the owner is never a member row of
// their own object, so RecoverAfterPropertyDelete above did not visit them.
// Their own suspended shared queue is recovered FIFO as well.
func (s *PropertyService) recoverSlotsAfterDelete(ctx context.Context, stores *txStores, actor, id uuid.UUID) error {
	if s.slots == nil {
		return nil
	}
	if err := s.slots.RecoverAfterPropertyDelete(ctx, stores.tx, id); err != nil {
		return fmt.Errorf("recover suspended memberships before delete: %w", err)
	}
	if err := s.slots.RecoverSuspended(ctx, stores.tx, actor); err != nil {
		return fmt.Errorf("recover owner suspended memberships after delete: %w", err)
	}
	return nil
}

// cleanupPropertyPhotos removes the deleted property's photo objects from
// storage after the commit; failures are logged, never propagated.
func (s *PropertyService) cleanupPropertyPhotos(ctx context.Context, id uuid.UUID, photos []domain.Photo) {
	for _, photo := range photos {
		key, err := photoStorageKey(id, photo.ID, photo.URL)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to derive storage key for photo cleanup",
				slog.String("photo_id", photo.ID.String()),
				slog.String("error", sanitizeError(err)),
			)
			continue
		}
		if err := s.photoStorage.Delete(ctx, key); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete property photo from storage",
				slog.String("photo_id", photo.ID.String()),
				slog.String("error", sanitizeError(err)),
			)
		}
	}
}

// archivePropertyInTx performs the core archive logic inside an existing
// transaction. The caller is responsible for committing or rolling back tx.
func (s *PropertyService) archivePropertyInTx(
	ctx context.Context,
	repo PropertyRepository,
	scope, id uuid.UUID,
) (domain.Property, error) {
	property, err := repo.GetByIDAndOwnerForUpdate(ctx, id, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("get property: %w", err)
	}

	if property.Status == domain.PropertyStatusArchived {
		return domain.Property{}, ErrAlreadyArchived
	}

	if err := repo.Archive(ctx, id, scope); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Property{}, ErrNotFound
		}
		return domain.Property{}, fmt.Errorf("archive property: %w", err)
	}

	archived, err := repo.GetByIDAndOwner(ctx, id, scope)
	if err != nil {
		return domain.Property{}, fmt.Errorf("reload archived property: %w", err)
	}

	return archived, nil
}

// ActivePropertyExists reports whether the property belongs to the scope
// owner and still occupies a tariff slot: the statuses that count toward the
// tariff limit are active and maintenance — the same set the excess-archive
// works on. The keep-choice validation of the cancel flow (issue #617) reads
// it inside its transaction.
func (s *PropertyService) ActivePropertyExists(ctx context.Context, tx transaction.Tx, scope, propertyID uuid.UUID) (bool, error) {
	property, err := s.repo.WithTx(tx).GetByIDAndOwner(ctx, propertyID, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get property: %w", err)
	}
	switch property.Status {
	case domain.PropertyStatusActive, domain.PropertyStatusMaintenance:
		return true, nil
	default:
		return false, nil
	}
}

// ArchiveExcessProperties archives active properties beyond the given limit,
// keeping the most recently updated properties, so the tariff limit is always
// enforced. When keepPropertyID is set (issue #617), the owner's chosen
// property takes a survivor slot as long as it is among the active ones — the
// cancel flow's promise that it survives the fall to basic; a stale id
// changes nothing. It returns the archived ids in restoration-priority
// order — the newest archived property first (the grace snapshot of ADR 0055).
func (s *PropertyService) ArchiveExcessProperties(
	ctx context.Context, tx transaction.Tx, scope uuid.UUID, limit int, keepPropertyID *uuid.UUID,
) ([]uuid.UUID, error) {
	if limit < 0 {
		return nil, nil
	}

	txRepo := s.repo.WithTx(tx)
	properties, err := txRepo.ListActiveByOwner(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("list active properties: %w", err)
	}

	// Keep the most recently updated properties; archive the rest.
	sort.SliceStable(properties, func(i, j int) bool {
		return properties[i].UpdatedAt.After(properties[j].UpdatedAt)
	})

	if keepPropertyID != nil {
		for i := range properties {
			if properties[i].ID == *keepPropertyID {
				chosen := properties[i]
				rest := make([]domain.Property, 0, len(properties)-1)
				rest = append(rest, properties[:i]...)
				rest = append(rest, properties[i+1:]...)
				properties = append([]domain.Property{chosen}, rest...)
				break
			}
		}
	}

	if len(properties) <= limit {
		return nil, nil
	}

	txAudit := s.audit.WithTx(tx)

	var archived []uuid.UUID
	for _, p := range properties[limit:] {
		_, err := s.archivePropertyInTx(ctx, txRepo, scope, p.ID)
		if err != nil {
			if errors.Is(err, ErrAlreadyArchived) {
				s.logger.WarnContext(ctx, "skipping auto-archive of property",
					"property_id", p.ID.String(),
					"owner_id", scope.String(),
					"error", err.Error())
				continue
			}
			return nil, fmt.Errorf("archive property %s: %w", p.ID, err)
		}
		archived = append(archived, p.ID)
		// Fail-safe: an audit failure aborts the billing operation that
		// triggered the auto-archive.
		if err := txAudit.Record(ctx, auditdomain.Entry{
			ActorID:    &scope,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyArchived,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &p.ID,
			Context:    map[string]any{"trigger": "billing_limit"},
		}); err != nil {
			return nil, fmt.Errorf("record audit: %w", err)
		}

		// Same as a manual archive (issue #163): archiving freed one tariff slot
		// for each recipient, so recover their oldest suspended memberships FIFO
		// in the same transaction.
		if s.slots != nil {
			if err := s.slots.RecoverSuspendedForProperty(ctx, tx, p.ID); err != nil {
				return nil, fmt.Errorf("recover suspended memberships after auto-archive: %w", err)
			}
			// Same as a manual archive: archiving one of the owner's OWN objects
			// also freed one of the owner's own tariff slots, so recover their own
			// suspended shared queue FIFO in the same transaction.
			if err := s.slots.RecoverSuspended(ctx, tx, scope); err != nil {
				return nil, fmt.Errorf("recover owner suspended memberships after auto-archive: %w", err)
			}
		}
	}
	return archived, nil
}

// RestoreGraceArchivedProperties unarchives the given properties of the owner
// — the ids billing archived at its grace entry and owes to restore (ADR
// 0055) — while the owner's active count stays under the limit; a negative
// limit is unlimited. The ids restore in the given (restoration-priority)
// order; an id gone or already active since the snapshot is skipped as
// settled. Every restoration mirrors a manual unarchive: the audit entry
// names the grace_restore trigger and the recipient slots are re-enforced for
// the property. Returns the ids it could not restore — the debt remainder
// still archived beyond the limit, in the given order (nil when everything
// settled).
func (s *PropertyService) RestoreGraceArchivedProperties(
	ctx context.Context, tx transaction.Tx, scope uuid.UUID, ids []uuid.UUID, limit int,
) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	txRepo := s.repo.WithTx(tx)
	txAudit := s.audit.WithTx(tx)
	count, err := txRepo.CountActiveByOwner(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("count active properties: %w", err)
	}

	for i, id := range ids {
		if limit >= 0 && count >= limit {
			// The limit is spent: the tail stays archived — the debt remainder.
			return ids[i:], nil
		}
		property, err := txRepo.GetByIDAndOwnerForUpdate(ctx, id, scope)
		if errors.Is(err, ErrNotFound) {
			// Deleted since the snapshot: nothing to restore.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get property %s: %w", id, err)
		}
		if property.Status != domain.PropertyStatusArchived {
			// Active again (restored manually since the snapshot): settled.
			continue
		}
		if err := txRepo.Unarchive(ctx, id, scope); err != nil {
			return nil, fmt.Errorf("unarchive property %s: %w", id, err)
		}
		count++
		if err := txAudit.Record(ctx, auditdomain.Entry{
			ActorID:    &scope,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyUnarchived,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &id,
			Context:    map[string]any{"trigger": "grace_restore"},
		}); err != nil {
			return nil, fmt.Errorf("record audit: %w", err)
		}
		// Same as a manual unarchive: the object re-enters every recipient's
		// tariff pool, so suspend any recipient already at their limit.
		if s.slots != nil {
			if err := s.slots.EnforceOnUnarchiveForProperty(ctx, tx, id); err != nil {
				return nil, fmt.Errorf("enforce recipient slot on grace restore: %w", err)
			}
		}
	}
	return nil, nil
}

func (s *PropertyService) UnarchiveProperty(ctx context.Context, actor, id uuid.UUID) (domain.Property, error) {
	role, err := s.resolveLifecycleProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, err
	}

	var unarchived domain.Property
	err = s.runInTx(ctx, func(stores *txStores) error {
		if err := ensurePropertyArchived(ctx, stores, actor, id); err != nil {
			return err
		}
		if err := ensureActivePropertySlot(ctx, stores, actor); err != nil {
			return err
		}

		if err := stores.repo.Unarchive(ctx, id, actor); err != nil {
			return fmt.Errorf("unarchive property: %w", err)
		}

		var err error
		unarchived, err = stores.repo.GetByIDAndOwner(ctx, id, actor)
		if err != nil {
			return fmt.Errorf("reload unarchived property: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPropertyUnarchived,
			EntityType: auditdomain.EntityProperty,
			EntityID:   &id,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, id, actor,
			sharedpolicy.HistoryActorRole(role), historydomain.PropertyUnarchived(id)); err != nil {
			return err
		}

		// Unarchiving the object re-enters every recipient's tariff pool: suspend any
		// recipient already at the limit so the object does not occupy a slot until
		// one frees up (issue #158, T4).
		if s.slots != nil {
			if err := s.slots.EnforceOnUnarchiveForProperty(ctx, stores.tx, id); err != nil {
				return fmt.Errorf("enforce recipient slot on unarchive: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}

	properties, err := s.withPhotos(ctx, unarchived)
	if err != nil {
		return domain.Property{}, err
	}
	properties[0].AccessRole = role
	return properties[0], nil
}

// ensurePropertyArchived loads the property row for update inside the
// unarchive transaction and requires the archived status: unarchiving a
// non-archived object is ErrNotArchived.
func ensurePropertyArchived(ctx context.Context, stores *txStores, owner, id uuid.UUID) error {
	property, err := stores.repo.GetByIDAndOwnerForUpdate(ctx, id, owner)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get property: %w", err)
	}
	if property.Status != domain.PropertyStatusArchived {
		return ErrNotArchived
	}
	return nil
}

// ensureActivePropertySlot enforces the tariff limit on active objects: the
// owner's active property count must stay under the subscription limit
// (CreateProperty, UnarchiveProperty).
func ensureActivePropertySlot(ctx context.Context, stores *txStores, owner uuid.UUID) error {
	limit, err := stores.limiter.ActivePropertyLimit(ctx, owner)
	if err != nil {
		return fmt.Errorf("get active property limit: %w", err)
	}

	count, err := stores.repo.CountActiveByOwner(ctx, owner)
	if err != nil {
		return fmt.Errorf("count active properties: %w", err)
	}
	if count >= limit {
		return ErrLimitExceeded
	}
	return nil
}

// NewPhotoTooLargeError reports a photo upload whose byte size exceeds
// MaxPhotoSize. The service size validation and the HTTP adapter's streaming
// bound share it so the message is identical wherever the check fires.
func NewPhotoTooLargeError(size int64) error {
	return fmt.Errorf("%w: file size %d exceeds %d bytes", ErrInvalidInput, size, MaxPhotoSize)
}

// AddPropertyPhoto validates and uploads a photo for the given property.
func (s *PropertyService) AddPropertyPhoto(
	ctx context.Context, actor, propertyID uuid.UUID, file io.Reader, filename, contentType string, size int64,
) (domain.Property, error) {
	if err := validatePhotoUpload(contentType, size); err != nil {
		return domain.Property{}, err
	}

	role, err := s.resolveEditableProperty(ctx, actor, propertyID)
	if err != nil {
		return domain.Property{}, err
	}

	var property domain.Property
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		property, err = lockEditableProperty(ctx, stores.repo, propertyID)
		if err != nil {
			return err
		}

		if err := ensurePhotoSlotAvailable(ctx, stores, propertyID); err != nil {
			return err
		}

		photoID, err := s.storePropertyPhoto(ctx, stores, propertyID, file, contentType, size)
		if err != nil {
			return err
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyPhotoAdded,
			EntityType: auditdomain.EntityPropertyPhoto,
			EntityID:   &photoID,
			Context:    map[string]any{"property_id": propertyID},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(role), historydomain.PropertyPhotoAdded(propertyID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, propertyID, true)

	photos, err := s.photoRepo.GetByPropertyID(ctx, propertyID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("load photos: %w", err)
	}
	property.Photos = photos

	return property, nil
}

// validatePhotoUpload checks the upload's content type and byte size against
// the photo constraints before any authorization or transaction work.
func validatePhotoUpload(contentType string, size int64) error {
	if _, ok := allowedPhotoContentTypes[contentType]; !ok {
		return fmt.Errorf("%w: unsupported content type %q", ErrInvalidInput, contentType)
	}
	if size > MaxPhotoSize {
		return NewPhotoTooLargeError(size)
	}
	return nil
}

// ensurePhotoSlotAvailable enforces the per-property photo limit.
func ensurePhotoSlotAvailable(ctx context.Context, stores *txStores, propertyID uuid.UUID) error {
	count, err := stores.photos.CountByPropertyID(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("count photos: %w", err)
	}
	if count >= maxPhotoCount {
		return ErrPhotoLimitReached
	}
	return nil
}

// storePropertyPhoto uploads the photo bytes to storage and persists the photo
// row, returning the new photo id. The storage key is derived from the id and
// the content-type extension, never from the client-supplied filename.
func (s *PropertyService) storePropertyPhoto(
	ctx context.Context, stores *txStores, propertyID uuid.UUID,
	file io.Reader, contentType string, size int64,
) (uuid.UUID, error) {
	photoID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate photo id: %w", err)
	}

	ext := allowedPhotoContentTypes[contentType]
	key := fmt.Sprintf("%s/%s/%s%s", photoKeyPrefix, propertyID.String(), photoID.String(), ext)
	photoURL, err := s.photoStorage.Upload(ctx, key, contentType, size, file)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upload photo: %w", err)
	}

	if _, err := stores.photos.Create(ctx, photoID, propertyID, photoURL); err != nil {
		return uuid.Nil, fmt.Errorf("create photo record: %w", err)
	}
	return photoID, nil
}

// DeletePropertyPhoto removes a photo record from the database and then deletes
// the file from storage on a best-effort basis. The DB record is the source of
// truth; if storage cleanup fails, the operation still succeeds and the orphan
// object is logged for later cleanup.
func (s *PropertyService) DeletePropertyPhoto(ctx context.Context, actor, propertyID, photoID uuid.UUID) error {
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone {
		return ErrNotFound
	}
	if !sharedpolicy.CanEdit(role) {
		return ErrForbidden
	}

	var photo domain.Photo
	err = s.runInTx(ctx, func(stores *txStores) error {
		property, err := stores.repo.GetByIDForUpdate(ctx, propertyID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get property: %w", err)
		}

		if property.Status == domain.PropertyStatusArchived {
			return ErrArchivedProperty
		}

		photo, err = stores.photos.GetByIDAndPropertyID(ctx, photoID, propertyID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get photo: %w", err)
		}

		if err := stores.photos.Delete(ctx, photoID); err != nil {
			return fmt.Errorf("delete photo record: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyPhotoDeleted,
			EntityType: auditdomain.EntityPropertyPhoto,
			EntityID:   &photoID,
			Context:    map[string]any{"property_id": propertyID},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(role), historydomain.PropertyPhotoDeleted(propertyID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.publishChanged(ctx, actor, propertyID, true)

	key, err := photoStorageKey(propertyID, photoID, photo.URL)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to derive storage key for photo cleanup",
			"property_id", propertyID.String(),
			"photo_id", photoID.String(),
			"error", err.Error())
		return nil
	}

	if err := s.photoStorage.Delete(ctx, key); err != nil {
		s.logger.WarnContext(ctx, "failed to delete photo from storage, record already removed",
			"property_id", propertyID.String(),
			"photo_id", photoID.String(),
			"key", key,
			"error", err.Error())
	}

	return nil
}

// auditFieldsKey is the audit context key listing the mutated fields.
const auditFieldsKey = "fields"

func sanitizeError(err error) string {
	return sanitize.Error(err)
}

func photoStorageKey(propertyID, photoID uuid.UUID, photoURL string) (string, error) {
	parsed, err := url.Parse(photoURL)
	if err != nil {
		return "", fmt.Errorf("parse photo url: %w", err)
	}
	ext := path.Ext(parsed.Path)
	if ext == "" {
		return "", errors.New("could not determine extension from photo url path")
	}
	return fmt.Sprintf("%s/%s/%s%s", photoKeyPrefix, propertyID.String(), photoID.String(), ext), nil
}

// withPhotos loads and attaches photos to the given properties.
func (s *PropertyService) withPhotos(ctx context.Context, properties ...domain.Property) ([]domain.Property, error) {
	if len(properties) == 0 {
		return properties, nil
	}

	ids := make([]uuid.UUID, len(properties))
	for i, p := range properties {
		ids[i] = p.ID
	}

	photosByProperty, err := s.photoRepo.GetByPropertyIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load photos: %w", err)
	}

	for i := range properties {
		properties[i].Photos = photosByProperty[properties[i].ID]
	}
	return properties, nil
}

func isUpdatableStatusTransition(from, to domain.PropertyStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case domain.PropertyStatusActive:
		return to == domain.PropertyStatusMaintenance
	case domain.PropertyStatusMaintenance:
		return to == domain.PropertyStatusActive
	case domain.PropertyStatusArchived:
		// Archived properties are terminal and cannot transition anywhere.
		return false
	}
	return false
}

// updatedPropertyFields lists the names of the fields a command changes. Only
// field names are audited, never their values.
func updatedPropertyFields(cmd UpdatePropertyCommand) []string {
	fields := make([]string, 0, 6)
	if cmd.Name != nil {
		fields = append(fields, "name")
	}
	if cmd.Type != nil {
		fields = append(fields, "type")
	}
	if cmd.Address != nil {
		fields = append(fields, "address")
	}
	if cmd.Description != nil {
		fields = append(fields, "description")
	}
	if cmd.Attributes != nil {
		fields = append(fields, "attributes")
	}
	if cmd.Status != nil {
		fields = append(fields, "status")
	}
	return fields
}

// resolvedPinnedAt computes the pin timestamp a PUT pin writes: a re-pin
// keeps the original pin time (the PUT's idempotency), a fresh pin takes the
// clock's now, an unpin clears it.
func resolvedPinnedAt(current *time.Time, pinned bool, now time.Time) *time.Time {
	if !pinned {
		return nil
	}
	if current != nil {
		return current
	}
	return &now
}

// propertyUpdateHistoryEntry composes the action journal entry for an edit
// from the old/new property states (ADR 0061 §4): a single changed field
// group gets its precise action id, several at once get the generic
// property.updated with the label list. The status is not part of the
// journal dictionary — a status-only edit yields no entry (ok=false).
func propertyUpdateHistoryEntry(propertyID uuid.UUID, before, after domain.Property) (historydomain.Entry, bool) {
	nameChanged := before.Name != after.Name
	addressChanged := before.Address != after.Address
	descriptionChanged := before.Description != after.Description
	typeChanged := before.Type != after.Type
	attrChanges := attributeChanges(before.Attributes, after.Attributes)

	labels := make([]string, 0, 5)
	if nameChanged {
		labels = append(labels, "название")
	}
	if addressChanged {
		labels = append(labels, "адрес")
	}
	if descriptionChanged {
		labels = append(labels, "описание")
	}
	if typeChanged {
		labels = append(labels, "тип")
	}
	if len(attrChanges) > 0 {
		labels = append(labels, "характеристики")
	}

	switch {
	case len(labels) == 0:
		return historydomain.Entry{}, false
	case len(labels) == 1:
		switch {
		case nameChanged:
			return historydomain.PropertyRenamed(propertyID, before.Name, after.Name), true
		case addressChanged:
			return historydomain.PropertyAddressChanged(propertyID, before.Address, after.Address), true
		case descriptionChanged:
			return historydomain.PropertyDescriptionChanged(propertyID, before.Description, after.Description), true
		case len(attrChanges) > 0:
			return historydomain.PropertyAttributesChanged(propertyID, attrChanges), true
		default: // The type field alone changed.
			return historydomain.PropertyUpdated(propertyID, labels), true
		}
	default:
		return historydomain.PropertyUpdated(propertyID, labels), true
	}
}

// attributeChanges diffs two attribute maps over the union of their keys;
// nil folds to an empty map so a cleared attribute set reads as removals.
func attributeChanges(before, after domain.Attributes) []historydomain.AttributeChange {
	if before == nil {
		before = domain.Attributes{}
	}
	if after == nil {
		after = domain.Attributes{}
	}
	changes := make([]historydomain.AttributeChange, 0, len(after))
	for key, newVal := range after {
		oldVal, existed := before[key]
		if existed && reflect.DeepEqual(oldVal, newVal) {
			continue
		}
		changes = append(changes, historydomain.AttributeChange{Key: key, Old: oldVal, New: newVal})
	}
	for key, oldVal := range before {
		if _, exists := after[key]; !exists {
			changes = append(changes, historydomain.AttributeChange{Key: key, Old: oldVal})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Key < changes[j].Key })
	return changes
}
