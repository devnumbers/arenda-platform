// Package application holds the properties use cases and ports: property lifecycle (create, archive, delete),
// photos and contacts.
package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	realtimedom "github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

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
	repo PropertyRepository
	// The storage port streams the object photo (ADR 0065): the upload lands in
	// the private storage under a fresh UUID key, the serving endpoints open
	// it back — no public URL ever exists.
	photoStorage       storageshared.PhotoStorage
	audit              auditapp.Recorder
	clock              clock.Clock
	policy             sharedpolicy.Policy
	sharedMemberships  SharedMemberships
	ownerNames         OwnerDisplayNameResolver
	ownerEmails        OwnerEmailResolver
	ownerPhotos        OwnerPhotoResolver
	suspendedShared    SuspendedSharedMemberships
	slots              RecipientSlotPolicy
	sharedDeleteMailer SharedMembersDeleteMailer
	rentalOccupancy    RentalOccupancyReader
	overdueOperations  OverdueOperationsReader
	rentalDeletion     RentalDeletionGuard
	archivePauses      ArchivePausesBridge
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
	var anchors []uuid.UUID
	if journaled {
		anchors = append(anchors, propertyID)
	}
	realtimeapp.Dispatch(ctx, s.realtime, actor,
		[]realtimedom.Change{realtimedom.On(realtimedom.EntityProperty, propertyID)}, anchors...)
}

// slotAccessFrames collects the slot policy's access pairs — recovered legs
// dirty their objects' participants views the same way freshly suspended ones
// do. The batch forwards raw — collapsing the repeated pairs is the carrier's
// contract (the per-call dedup of the stream Publisher's EntityChanged), the
// same reliance as the access seam's recovery dispatch. No journal anchors —
// the slot coordinator journals no recovery row (карта #714, #716; ADR 0062 §3).
func slotAccessFrames(legs []accessdomain.Membership) []realtimedom.Change {
	changed := make([]realtimedom.Change, 0, len(legs))
	for _, m := range legs {
		changed = append(changed, realtimedom.On(realtimedom.EntityAccess, m.PropertyID))
	}
	return changed
}

// publishSlotAccess hands the slot policy's access pairs (recovered —
// archive/delete seams; suspended — unarchive) to the realtime carrier
// (карта #714, #716; ADR 0062 §3). The caller captures the legs inside its
// transaction and invokes this after the commit; best-effort, a nil carrier
// keeps the pre-#716 silence.
func (s *PropertyService) publishSlotAccess(ctx context.Context, actor uuid.UUID, legs []accessdomain.Membership) {
	if len(legs) == 0 {
		return
	}
	realtimeapp.Dispatch(ctx, s.realtime, actor, slotAccessFrames(legs))
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

// SetOwnerPhotoResolver injects the adapter that resolves a property
// owner's profile photo path (ADR 0065, решение #1286). Optional: when not
// set, responses carry no owner photo.
func (s *PropertyService) SetOwnerPhotoResolver(resolver OwnerPhotoResolver) {
	s.ownerPhotos = resolver
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

// SetArchivePausesBridge injects the payments-context bridge of the
// archive round-trip's system pauses (тикет #1342): the archive pauses the
// object's live rules, the restore closes only the auto_archive ones.
// Optional: when not set, the lifecycle changes skip the pauses (a test
// wiring).
func (s *PropertyService) SetArchivePausesBridge(bridge ArchivePausesBridge) {
	s.archivePauses = bridge
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
// embedded factory (ADR 0033 γ-factory); repo additionally serves the
// non-transactional reads and the externally-owned transaction of
// ArchiveExcessProperties. The photos port streams the object photo
// (ADR 0065).
func NewPropertyService(
	repo PropertyRepository,
	photoStorage storageshared.PhotoStorage,
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

	// Автогенерация названия (#1001): пустое имя выводится из типа. Читается
	// мимо транзакции: имена не уникальны, гонка двух созданий даёт повтор
	// серийника без какого-либо вреда.
	name := cmd.Name
	if strings.TrimSpace(name) == "" {
		if name, err = autoName(ctx, s.repo, actor, propertyType, false); err != nil {
			return domain.Property{}, err
		}
	}

	property, err := domain.NewProperty(actor, name, cmd.Address, cmd.Description, propertyType, attrs)
	if err != nil {
		return domain.Property{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	now := s.clock.Now()
	property.CreatedAt = now
	property.UpdatedAt = now

	var created domain.Property
	err = s.runInTx(ctx, func(stores *txStores) error {
		if err := s.ensureCreationSlot(ctx, stores, actor); err != nil {
			return err
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

	// The «Архив» gate rides the list read (issue #1233): the owner's own
	// archived rows only — shared properties contribute nothing.
	archivedCount, err := s.repo.CountArchivedByOwner(ctx, actor)
	if err != nil {
		return PropertiesPage{}, fmt.Errorf("count archived properties: %w", err)
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

	// The actor's personal pins ride the assembled list (the cutover
	// #1307): the order below is the actor's, never another participant's.
	if err := s.overlayActorPins(ctx, actor, properties); err != nil {
		return PropertiesPage{}, err
	}

	items := s.pinnedFirst(properties)
	return PropertiesPage{Items: items, Today: today, ArchivedCount: archivedCount, SuspendedShared: suspended}, nil
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
	type ownerContact struct {
		name     string
		photoURL string
	}
	resolved := make(map[uuid.UUID]ownerContact)
	for i := range properties {
		if properties[i].AccessRole == sharedpolicy.RoleOwner {
			continue
		}
		ownerID := properties[i].OwnerID
		contact, ok := resolved[ownerID]
		if !ok {
			name, err := s.ownerNames.DisplayName(ctx, ownerID)
			if err != nil {
				s.logger.WarnContext(ctx, "failed to resolve owner display name",
					slog.String("owner_id", ownerID.String()),
					slog.String("error", sanitizeError(err)),
				)
			}
			contact = ownerContact{name: name}
			if s.ownerPhotos != nil {
				photoURL, photoErr := s.ownerPhotos.GetPhotoURL(ctx, ownerID)
				if photoErr != nil {
					s.logger.WarnContext(ctx, "failed to resolve owner photo",
						slog.String("owner_id", ownerID.String()),
						slog.String("error", sanitizeError(photoErr)),
					)
				}
				contact.photoURL = photoURL
			}
			resolved[ownerID] = contact
		}
		properties[i].OwnerName = contact.name
		properties[i].OwnerPhotoURL = contact.photoURL
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
// Stable so equal pins never swap. The rows carry the actor's personal
// pinnedAt (the overlay below); this re-sort extends the same rule over
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

// overlayActorPins fills the actor's personal pins over the assembled list
// (the cutover #1307): the lists carry pinnedAt of the actor — own and
// shared rows alike — and pinnedFirst orders by it. The table carries no
// pin column anymore (the contract #1311 dropped it); the actor's
// user_property_pins rows are the only pin source.
func (s *PropertyService) overlayActorPins(ctx context.Context, actor uuid.UUID, properties []domain.Property) error {
	if len(properties) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(properties))
	for i := range properties {
		ids[i] = properties[i].ID
	}
	pins, err := s.repo.ActorPins(ctx, actor, ids)
	if err != nil {
		return fmt.Errorf("read actor pins: %w", err)
	}
	for i := range properties {
		if at, ok := pins[properties[i].ID]; ok {
			properties[i].PinnedAt = &at
		}
	}
	return nil
}

// overlayActorPin fills the actor's personal pin on a single property read
// (the cutover #1307): the detail carries pinnedAt of the actor.
func (s *PropertyService) overlayActorPin(ctx context.Context, actor uuid.UUID, property *domain.Property) error {
	at, err := s.repo.ActorPin(ctx, actor, property.ID)
	if err != nil {
		return fmt.Errorf("read actor pin: %w", err)
	}
	property.PinnedAt = at
	return nil
}

// overlayActorPinDegrading fills the actor's personal pin on a mutation
// response (the cutover #1307): by the time it runs the mutation is
// committed and its frame published — a failed pin read degrades to an
// unpinned answer (the reader's other surfaces re-read the truth on the
// published frame) instead of failing the answered call. The read paths
// keep the strict overlayActorPin: there the pin state is the answer.
func (s *PropertyService) overlayActorPinDegrading(ctx context.Context, actor uuid.UUID, property *domain.Property) {
	at, err := s.repo.ActorPin(ctx, actor, property.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to overlay actor pin on mutation response",
			slog.String("property_id", property.ID.String()),
			slog.String("error", sanitizeError(err)),
		)
		return
	}
	property.PinnedAt = at
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

	items := s.pinnedFirst(properties)
	// The archived list is the archived count: unpaginated, the listed rows
	// are the owner's archived set (issue #1233) — no second COUNT query.
	return PropertiesPage{Items: items, Today: today, ArchivedCount: len(items)}, nil
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
	// The search rows carry the actor's personal pins (the cutover #1307) —
	// the cards render the same pin mark the hub list does. The keyset walk
	// stays untouched: the cursor travels (name, id), not the pin order.
	if err := s.overlayActorPins(ctx, actor, properties); err != nil {
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
	// The detail carries the actor's personal pin (the cutover #1307): the
	// pin mark and the «Убрать из основных» action read the actor's row.
	if err := s.overlayActorPin(ctx, actor, &property); err != nil {
		return domain.Property{}, err
	}

	// The actor's access context for the sharing banner (issue T11): the role
	// is always reported; the owner's public contacts resolve only for
	// recipients.
	property.AccessRole = role
	s.resolveOwnerContacts(ctx, role, &property)

	return property, nil
}

// resolveOwnerContacts fills the owner's display name, account email and
// photo path on the detail read — recipients only (issue T11, Figma
// 2200-97365, ADR 0065 решение #1286); every resolver is optional and
// failure-degrading, a failed lookup leaves the field empty.
func (s *PropertyService) resolveOwnerContacts(ctx context.Context, role sharedpolicy.Role, property *domain.Property) {
	if role == sharedpolicy.RoleOwner {
		return
	}
	if s.ownerNames != nil {
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
	if s.ownerEmails != nil {
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
	if s.ownerPhotos != nil {
		photoURL, err := s.ownerPhotos.GetPhotoURL(ctx, property.OwnerID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to resolve owner photo",
				slog.String("property_id", property.ID.String()),
				slog.String("error", sanitizeError(err)),
			)
		} else {
			property.OwnerPhotoURL = photoURL
		}
	}
}

func (s *PropertyService) UpdateProperty(ctx context.Context, actor, id uuid.UUID, cmd UpdatePropertyCommand) (domain.Property, error) {
	role, err := s.resolveEditableProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, err
	}

	var updated domain.Property
	var journaled bool
	err = s.runInTx(ctx, func(stores *txStores) error {
		property, err := lockPropertyForWrite(ctx, stores.repo, id)
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
		// The maintenance flip journals its own row (протокол D2, карта
		// #1324; ADR 0061 §3) — the guard is the FACTUAL inequality, and a
		// mixed PATCH writes two rows (the CompleteRental precedent).
		maintenanceJournaled, err := s.recordMaintenanceHistory(ctx, stores, id, actor, role, before, updated)
		if err != nil {
			return err
		}
		journaled = journaled || maintenanceJournaled
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, id, journaled)

	// The response carries the actor's personal pin (the cutover #1307) —
	// the edit did not touch it, the reader's row is the truth.
	s.overlayActorPinDegrading(ctx, actor, &updated)

	updated.AccessRole = role
	return updated, nil
}

// recordMaintenanceHistory journals the active↔maintenance flip as its own
// row (протокол D2, карта #1324; ADR 0061 §3): started on the way in,
// finished on the way out. The guard is the FACTUAL inequality —
// isUpdatableStatusTransition admits from==to (the idempotent PUT), and a
// no-op action writes no row. Reports whether a row was written so the
// caller's journaled flag (and with it the SSE feed publication) follows the
// journal.
func (s *PropertyService) recordMaintenanceHistory(
	ctx context.Context, stores *txStores, propertyID, actor uuid.UUID,
	role sharedpolicy.Role, before, after domain.Property,
) (bool, error) {
	if before.Status == after.Status {
		return false, nil
	}
	entry := historydomain.PropertyMaintenanceFinished(propertyID)
	if after.Status == domain.PropertyStatusMaintenance {
		entry = historydomain.PropertyMaintenanceStarted(propertyID)
	}
	if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
		sharedpolicy.HistoryActorRole(role), entry); err != nil {
		return false, err
	}
	return true, nil
}

// SetPropertyPin writes the actor's personal pin (PUT pin, ticket #577; the
// personal cutover #1307, D11): one insert/delete of the actor's
// user_property_pins row inside the transaction under the view capability —
// Owner, Full Access and Viewer alike; no access and a suspended membership
// are ErrNotFound, an archived property is ErrArchivedProperty. A re-pin
// keeps the original pin time (the PUT's idempotency — the order among the
// pinned never shifts); unpinning deletes the row. The pin is a personal
// state: no history row is written (D17) and no realtime frame is published
// (D18) — the actor's other tabs re-read on their next load; the admin
// audit keeps recording the PUT.
func (s *PropertyService) SetPropertyPin(ctx context.Context, actor, id uuid.UUID, pinned bool) (domain.Property, error) {
	role, err := s.resolveViewableProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, err
	}

	var updated domain.Property
	err = s.runInTx(ctx, func(stores *txStores) error {
		property, err := lockPropertyForWrite(ctx, stores.repo, id)
		if err != nil {
			return err
		}

		var pinnedAt *time.Time
		if pinned {
			// The atomic personal-pin insert: a re-pin keeps the original
			// pin time, a fresh pin takes the clock's now — the statement
			// returns the effective one either way.
			at, err := stores.repo.InsertActorPin(ctx, actor, id, s.clock.Now())
			if err != nil {
				return fmt.Errorf("insert actor pin: %w", err)
			}
			pinnedAt = &at
		} else if err := stores.repo.DeleteActorPin(ctx, actor, id); err != nil {
			return fmt.Errorf("delete actor pin: %w", err)
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

		updated = property
		updated.PinnedAt = pinnedAt
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}

	updated.AccessRole = role
	return updated, nil
}

// resolveViewableProperty authorizes the personal pin write (SetPropertyPin,
// the cutover #1307): the actor needs the view capability — a viewer pins
// their own order (D11). No access and a suspended membership both map to
// ErrNotFound so the existence of an object is never revealed (issue #156,
// T3): unlike the detail read, the pin endpoint carries no suspended signal
// — a PUT reveals nothing and answers the privacy 404.
func (s *PropertyService) resolveViewableProperty(ctx context.Context, actor, id uuid.UUID) (sharedpolicy.Role, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanView(role) {
		return "", ErrNotFound
	}
	return role, nil
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

// autoName composes the generated property name (#1001, #1078): the serial —
// the object's position among the owner's properties of the type (active and
// archived alike, deleted rows gone) — over the domain phrase map; the first
// property of the type carries no serial. The selfCounted flag tells whether
// the repo count already includes the object itself: the create path counts
// before the insert, and the update path counts DB rows that still carry the
// old type — so it is true only when the PATCH leaves the type unchanged.
// The repo handle is the caller's store: the plain repo on the create path,
// the transactional store inside applyPropertyUpdate.
func autoName(
	ctx context.Context,
	repo PropertyRepository,
	scope uuid.UUID,
	propertyType domain.PropertyType,
	selfCounted bool,
) (string, error) {
	count, err := repo.CountByOwnerAndType(ctx, scope, propertyType)
	if err != nil {
		return "", fmt.Errorf("count properties for auto name: %w", err)
	}
	if !selfCounted {
		count++
	}
	return domain.AutoPropertyName(propertyType, count), nil
}

// lockPropertyForWrite loads the property row for update inside a write
// transaction and rejects archived objects: an archived property is read-only
// history. The personal pin write rides the same guard (the cutover #1307):
// the right to write differs — pin needs only the view capability, resolved
// by resolveViewableProperty — but the archived verdict is common.
func lockPropertyForWrite(ctx context.Context, repo PropertyRepository, id uuid.UUID) (domain.Property, error) {
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
// re-validates the aggregate; only the provided fields change. The type is
// applied before the name: clearing the name (ticket #1001) regenerates it
// from the type, which the same PATCH may have just changed.
func applyPropertyUpdate(ctx context.Context, stores *txStores, property *domain.Property, cmd UpdatePropertyCommand) error {
	if cmd.Type != nil {
		propertyType, err := domain.ParsePropertyType(*cmd.Type)
		if err != nil {
			return fmt.Errorf("%w: invalid property type: %w", ErrInvalidInput, err)
		}
		property.Type = propertyType
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			// Очистка названия при правке (#1001, #1078): регенерация из
			// типа. Пока строка в БД не перезаписана, счёт включает сам
			// объект только при неизменном типе (строка уже нового типа
			// не станет до UPDATE) — серийник и есть позиция среди
			// однотипных; при смене типа тем же PATCH сам объект в счёт
			// нового типа не попадает.
			name, err := autoName(ctx, stores.repo, property.OwnerID, property.Type, cmd.Type == nil)
			if err != nil {
				return err
			}
			property.Name = name
		} else {
			property.Name = *cmd.Name
		}
	}
	if cmd.Address != nil {
		property.Address = *cmd.Address
	}
	if cmd.Description != nil {
		property.Description = *cmd.Description
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
	// The recovery legs captured inside the transaction; their access pairs
	// dispatch post-commit next to publishChanged (карта #714, #716; ADR 0062
	// §3).
	var recovered []accessdomain.Membership
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		archived, err = s.archivePropertyInTx(ctx, stores.repo, actor, id)
		if err != nil {
			return err
		}

		// The archive pauses the object's live payment rules (тикет #1342):
		// the system auto_archive pause in the same transaction — the
		// reactions to the object's lifecycle, the rent-managed gate does
		// not apply (ADR 0053 §3); the payment journals mark the origin.
		if err := s.pausePaymentsOnArchive(ctx, stores.tx, actor, id, actor, role); err != nil {
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
		// (issue #158, T4). The reactivated legs are captured: their access pairs
		// are the properties-side publication of the recovery, dispatched
		// post-commit below like publishChanged (карта #714, #716; ADR 0062 §3).
		if s.slots != nil {
			legs, err := s.slots.RecoverSuspendedForProperty(ctx, stores.tx, id)
			if err != nil {
				return fmt.Errorf("recover suspended memberships after archive: %w", err)
			}
			recovered = append(recovered, legs...)
			// Archiving one of the owner's OWN objects also freed one of the owner's
			// own tariff slots: the owner is never a member row of their own object,
			// so RecoverSuspendedForProperty above did not visit them. Recover their
			// own suspended shared queue FIFO in the same transaction — its legs
			// live in other owners' objects and ride the same recovery dispatch.
			legs, err = s.slots.RecoverSuspended(ctx, stores.tx, actor)
			if err != nil {
				return fmt.Errorf("recover owner suspended memberships after archive: %w", err)
			}
			recovered = append(recovered, legs...)
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, id, true)
	s.publishSlotAccess(ctx, actor, recovered)

	archived.AccessRole = role
	return archived, nil
}

// DeleteProperty removes the property together with all its data: the FKs
// cascade rentals, payments, operations, tasks, photos and bound contacts
// off the property row (contacts: ADR 0054 §2 as amended 2026-10-02,
// ticket #1049). ADR 0049 closed the ADR 0025 detach branch — deletion is
// total, there is no mode (issue #629).
func (s *PropertyService) DeleteProperty(
	ctx context.Context,
	actor, id uuid.UUID,
) error {
	if _, err := s.resolveLifecycleProperty(ctx, actor, id); err != nil {
		return err
	}

	// The deleted property's name, photo key, former members' emails and the
	// slot recovery's legs escape the work closure for the post-commit
	// cleanup.
	var (
		property           domain.Property
		photoKey           *string
		formerMemberEmails []string
		recovered          []accessdomain.Membership
	)
	err := s.runInTx(ctx, func(stores *txStores) error {
		var err error
		property, err = lockDeletableProperty(ctx, stores, actor, id)
		if err != nil {
			return err
		}
		photoKey = property.PhotoKey

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

		recovered, err = s.recoverSlotsAfterDelete(ctx, stores, actor, id)
		if err != nil {
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

	// Post-commit cleanup never fails the delete itself; the property row is
	// already locked here, so its photo key was read in the same closure.
	if photoKey != nil {
		s.cleanupPropertyPhoto(ctx, *photoKey)
	}
	s.notifyPropertyDeleted(ctx, id, property.Name, formerMemberEmails)
	// The deleted object's own row dispatches nothing by construction — the
	// derived access resolves to nobody — but the recovery's reactivated legs
	// live in other owners' objects, and their access pairs dispatch here
	// (карта #714, #716; ADR 0062 §3).
	s.publishSlotAccess(ctx, actor, recovered)

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
// Their own suspended shared queue is recovered FIFO as well. The reactivated
// memberships are returned so the caller dispatches their access pairs
// post-commit (карта #714, #716; ADR 0062 §3).
func (s *PropertyService) recoverSlotsAfterDelete(
	ctx context.Context, stores *txStores, actor, id uuid.UUID,
) ([]accessdomain.Membership, error) {
	if s.slots == nil {
		return nil, nil
	}
	var recovered []accessdomain.Membership
	legs, err := s.slots.RecoverAfterPropertyDelete(ctx, stores.tx, id)
	if err != nil {
		return nil, fmt.Errorf("recover suspended memberships before delete: %w", err)
	}
	recovered = append(recovered, legs...)
	legs, err = s.slots.RecoverSuspended(ctx, stores.tx, actor)
	if err != nil {
		return nil, fmt.Errorf("recover owner suspended memberships after delete: %w", err)
	}
	recovered = append(recovered, legs...)
	return recovered, nil
}

// cleanupPropertyPhoto removes the property's photo object from storage
// after the commit; failures are logged as orphans, never propagated.
func (s *PropertyService) cleanupPropertyPhoto(ctx context.Context, key string) {
	if err := s.photoStorage.Delete(ctx, key); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete property photo from storage",
			slog.String("key", key),
			slog.String("error", sanitizeError(err)),
		)
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

// pausePaymentsOnArchive drives the payments bridge's pause leg inside the
// archive transaction (тикет #1342); a nil bridge (a test wiring) skips.
func (s *PropertyService) pausePaymentsOnArchive(
	ctx context.Context, tx transaction.Tx, scope, propertyID, actor uuid.UUID, role sharedpolicy.Role,
) error {
	if s.archivePauses == nil {
		return nil
	}
	if err := s.archivePauses.PauseAllForArchive(ctx, tx, scope, propertyID, actor, role); err != nil {
		return fmt.Errorf("pause payments on archive: %w", err)
	}
	return nil
}

// resumePaymentsOnRestore drives the bridge's reverse leg inside the
// restore transaction (тикет #1342); a nil bridge (a test wiring) skips.
func (s *PropertyService) resumePaymentsOnRestore(
	ctx context.Context, tx transaction.Tx, scope, propertyID, actor uuid.UUID, role sharedpolicy.Role,
) error {
	if s.archivePauses == nil {
		return nil
	}
	if err := s.archivePauses.ResumeArchivePauses(ctx, tx, scope, propertyID, actor, role); err != nil {
		return fmt.Errorf("resume payments on restore: %w", err)
	}
	return nil
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
	// The recovery legs captured across the loop; their access pairs dispatch
	// once after it — the whole auto-archive batch emits one frame per
	// restored object (карта #714, #716; ADR 0062 §3).
	var recovered []accessdomain.Membership
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
		// in the same transaction. The reactivated legs are captured: their
		// access pairs are the properties-side publication of the recovery
		// (карта #714, #716; ADR 0062 §3), dispatched after the loop below.
		if s.slots != nil {
			legs, err := s.slots.RecoverSuspendedForProperty(ctx, tx, p.ID)
			if err != nil {
				return nil, fmt.Errorf("recover suspended memberships after auto-archive: %w", err)
			}
			recovered = append(recovered, legs...)
			// Same as a manual archive: archiving one of the owner's OWN objects
			// also freed one of the owner's own tariff slots, so recover their own
			// suspended shared queue FIFO in the same transaction — its legs live
			// in other owners' objects and ride the same recovery dispatch.
			legs, err = s.slots.RecoverSuspended(ctx, tx, scope)
			if err != nil {
				return nil, fmt.Errorf("recover owner suspended memberships after auto-archive: %w", err)
			}
			recovered = append(recovered, legs...)
		}
	}
	// The archiver bridge runs on the billing phase's transaction: the phase
	// commits right after the archiver hands its outcome back, so the
	// dispatch sits as close to the commit as the bridge boundary allows —
	// the frames are coarse and best-effort, a phase rollback past this point
	// leaves at most a spurious refetch (ADR 0062 §5).
	s.publishSlotAccess(ctx, scope, recovered)
	return archived, nil
}

// RestoreGraceArchivedProperties unarchives the given properties of the owner
// — the ids billing archived at its grace entry and owes to restore (ADR
// 0055) — while the owner's active count stays under the limit; a negative
// limit is unlimited. The ids restore in the given (restoration-priority)
// order; an id gone or already active since the snapshot is skipped as
// settled. Every restoration mirrors a manual unarchive: the audit entry
// names the grace_restore trigger, the recipient slots are re-enforced for
// the property and the owner's own pool growth is re-enforced with the same
// trigger (D13). Returns the ids it could not restore — the debt remainder
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
		restored, err := s.restoreGraceProperty(ctx, tx, txRepo, txAudit, scope, id)
		if err != nil {
			return nil, err
		}
		if restored {
			count++
		}
	}
	return nil, nil
}

// restoreGraceProperty unarchives one snapshotted property inside the
// billing transaction: the row lock, the gone/already-active skips, the
// payments resume, the audit entry and the recipient-slot enforcement
// (ADR 0055). It reports whether the property was actually restored — a
// false with a nil error is the gone-or-settled skip.
func (s *PropertyService) restoreGraceProperty(
	ctx context.Context, tx transaction.Tx, txRepo PropertyRepository, txAudit auditapp.Recorder,
	scope, id uuid.UUID,
) (bool, error) {
	property, err := txRepo.GetByIDAndOwnerForUpdate(ctx, id, scope)
	if errors.Is(err, ErrNotFound) {
		// Deleted since the snapshot: nothing to restore.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get property %s: %w", id, err)
	}
	if property.Status != domain.PropertyStatusArchived {
		// Active again (restored manually since the snapshot): settled.
		return false, nil
	}
	if err := txRepo.Unarchive(ctx, id, scope); err != nil {
		return false, fmt.Errorf("unarchive property %s: %w", id, err)
	}
	// The grace restore is the manual unarchive's mirror (ADR 0055): the
	// return closes only the system auto_archive pauses (тикет #1342) —
	// a property restored with its own pauses left open would strand
	// them with no reason while the object is active.
	if err := s.resumePaymentsOnRestore(ctx, tx, scope, id, scope, sharedpolicy.RoleOwner); err != nil {
		return false, fmt.Errorf("resume payments on grace restore %s: %w", id, err)
	}
	if err := txAudit.Record(ctx, auditdomain.Entry{
		ActorID:    &scope,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPropertyUnarchived,
		EntityType: auditdomain.EntityProperty,
		EntityID:   &id,
		Context:    map[string]any{"trigger": "grace_restore"},
	}); err != nil {
		return false, fmt.Errorf("record audit: %w", err)
	}
	// Same as a manual unarchive: the object re-enters every recipient's
	// tariff pool, so suspend any recipient already at their limit. The
	// suspended legs are deliberately not published: the restore runs on
	// the billing phase of the background worker, outside the v1
	// realtime boundary (ADR 0062 §3, #716) — the owner learns the
	// participants state through the next reread of the screen.
	if s.slots != nil {
		if _, err := s.slots.EnforceOnUnarchiveForProperty(ctx, tx, id); err != nil {
			return false, fmt.Errorf("enforce recipient slot on grace restore: %w", err)
		}
		// The owner's own pool grew too — the restored object re-entered it
		// (D13, #1297): the full pool is re-enforced (the excess shared
		// memberships suspended, the restored own objects stay). The legs
		// are dropped for the same reason as above: a background billing
		// phase publishes nothing.
		if _, err := s.slots.EnforceRecipientPoolGrowth(ctx, tx, scope, "grace_restore"); err != nil {
			return false, fmt.Errorf("enforce recipient pool growth on grace restore: %w", err)
		}
	}
	return true, nil
}

func (s *PropertyService) UnarchiveProperty(ctx context.Context, actor, id uuid.UUID) (domain.Property, error) {
	role, err := s.resolveLifecycleProperty(ctx, actor, id)
	if err != nil {
		return domain.Property{}, err
	}

	var unarchived domain.Property
	// The suspended legs captured inside the transaction; their access pairs
	// dispatch post-commit next to publishChanged (карта #714, #716; ADR 0062
	// §3).
	var suspended []accessdomain.Membership
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

		// The return from the archive closes ONLY the system auto_archive
		// pauses (тикет #1342): the owner's manual pauses stay open.
		if err := s.resumePaymentsOnRestore(ctx, stores.tx, actor, id, actor, role); err != nil {
			return err
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
		// one frees up (issue #158, T4). The suspended legs are captured: their
		// access pairs are the properties-side publication of the suspension,
		// dispatched post-commit below like publishChanged — each suspended leg
		// dirties its object's participants view the same way a restored one does
		// (карта #714, #716; ADR 0062 §3).
		if s.slots != nil {
			legs, err := s.slots.EnforceOnUnarchiveForProperty(ctx, stores.tx, id)
			if err != nil {
				return fmt.Errorf("enforce recipient slot on unarchive: %w", err)
			}
			suspended = append(suspended, legs...)
			// The actor's OWN pool grew too — the unarchived object re-entered it
			// (D13, #1297): the owner is never a member row of their own object,
			// so the per-property entry above does not visit them as a recipient.
			// The full pool is re-enforced: the excess shared memberships beyond
			// the limit are suspended (the oldest first, the returning own object
			// stays), never the own candidates. The legs ride the same post-commit
			// dispatch — a suspended shared object is exactly the blur-card state
			// the recipient's clients must refetch.
			legs, err = s.slots.EnforceRecipientPoolGrowth(ctx, stores.tx, actor, "unarchive")
			if err != nil {
				return fmt.Errorf("enforce recipient pool growth on unarchive: %w", err)
			}
			suspended = append(suspended, legs...)
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	s.publishChanged(ctx, actor, id, true)
	s.publishSlotAccess(ctx, actor, suspended)

	// D16: the personal pins survived the archive untouched — the returned
	// object carries the actor's pin again (the cutover #1307).
	s.overlayActorPinDegrading(ctx, actor, &unarchived)

	unarchived.AccessRole = role
	return unarchived, nil
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

// ensureCreationSlot is CreateProperty's limit gate — the honest refusal of
// #1300 Q2 (D13): the budget is the recipient's FULL pool (own active objects
// plus the active shared memberships they hold), and a full pool refuses the
// creation outright with ErrLimitExceeded — creation returns nothing, so
// silently suspending someone else's active access to fit the new object in is
// less predictable than an honest «лимит превышен». The full-pool predicate
// lives in the slot policy; without it wired (pre-T4 behaviour) the own-count
// check is the fallback.
func (s *PropertyService) ensureCreationSlot(ctx context.Context, stores *txStores, actor uuid.UUID) error {
	if s.slots == nil {
		return ensureActivePropertySlot(ctx, stores, actor)
	}
	full, err := s.slots.RecipientPoolFull(ctx, stores.tx, actor)
	if err != nil {
		return fmt.Errorf("check recipient pool: %w", err)
	}
	if full {
		return ErrLimitExceeded
	}
	return nil
}

// PhotoDescriptor resolves the object photo's identity for the serving
// endpoint (ADR 0065): the key powers the ETag — a replacement mints a new
// UUID key, so a 304 never lies about the bytes. Access follows ADR 0028:
// every role that can view the object can view its photo.
func (s *PropertyService) PhotoDescriptor(ctx context.Context, actor, id uuid.UUID) (key, contentType string, err error) {
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return "", "", fmt.Errorf("resolve role: %w", err)
	}
	switch sharedpolicy.GateFor(role, sharedpolicy.CanView) {
	case sharedpolicy.GateAllow:
	case sharedpolicy.GateForbidden:
		return "", "", ErrForbidden
	default: // GateNone, GateSuspended.
		return "", "", s.accessFailure(role)
	}

	property, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	if property.PhotoKey == nil {
		return "", "", ErrPhotoNotFound
	}
	return *property.PhotoKey, derefString(property.PhotoContentType), nil
}

// OpenPropertyPhoto opens the object photo (ADR 0065): the body, its byte
// size, the stored content type and the key (the ETag source). A column key
// without an object is an orphan — the honest ErrPhotoNotFound.
func (s *PropertyService) OpenPropertyPhoto(
	ctx context.Context, actor, id uuid.UUID,
) (body io.ReadCloser, size int64, contentType, key string, err error) {
	// One policy resolution and one row read: the descriptor path already
	// gated the actor, this repeat covers the 200 path end to end.
	role, err := s.policy.RoleForProperty(ctx, actor, id)
	if err != nil {
		return nil, 0, "", "", fmt.Errorf("resolve role: %w", err)
	}
	switch sharedpolicy.GateFor(role, sharedpolicy.CanView) {
	case sharedpolicy.GateAllow:
	case sharedpolicy.GateForbidden:
		return nil, 0, "", "", ErrForbidden
	default: // GateNone, GateSuspended.
		return nil, 0, "", "", s.accessFailure(role)
	}

	property, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, 0, "", "", err
	}
	if property.PhotoKey == nil {
		return nil, 0, "", "", ErrPhotoNotFound
	}
	body, size, contentType, err = s.photoStorage.Open(ctx, *property.PhotoKey)
	if err != nil {
		if errors.Is(err, storageshared.ErrNotFound) {
			return nil, 0, "", "", ErrPhotoNotFound
		}
		return nil, 0, "", "", fmt.Errorf("open property photo: %w", err)
	}
	return body, size, contentType, *property.PhotoKey, nil
}

// SetPropertyPhoto replaces the object photo (ADR 0065, one image per
// entity): the processed upload lands under a fresh UUID key, the columns
// flip in the same transaction, and the previous object is removed
// best-effort after the commit. Edits are gated by the ADR 0028 edit
// capability; an archived object accepts no photo changes, its photo stays
// readable.
func (s *PropertyService) SetPropertyPhoto(
	ctx context.Context, actor, propertyID uuid.UUID, processed photo.Processed,
) (domain.Property, error) {
	key, err := photo.NewKey(processed.ContentType)
	if err != nil {
		return domain.Property{}, err
	}

	role, err := s.resolveEditableProperty(ctx, actor, propertyID)
	if err != nil {
		return domain.Property{}, err
	}

	var (
		property domain.Property
		oldKey   *string
	)
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		property, err = lockPropertyForWrite(ctx, stores.repo, propertyID)
		if err != nil {
			return err
		}
		oldKey = property.PhotoKey

		if err := s.photoStorage.Put(ctx, key, bytes.NewReader(processed.Data), processed.ContentType, processed.Size); err != nil {
			return fmt.Errorf("put property photo: %w", err)
		}

		property, err = stores.repo.SetPropertyPhoto(ctx, propertyID, property.OwnerID, &key, &processed.ContentType)
		if err != nil {
			return fmt.Errorf("set property photo: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyPhotoAdded,
			EntityType: auditdomain.EntityPropertyPhoto,
			EntityID:   &propertyID,
			Context:    map[string]any{"property_id": propertyID},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(role), historydomain.PropertyPhotosChanged(propertyID, property.Name, true)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// The new object may have landed before a later step failed: it is
		// unreferenced — remove it the same best-effort way as the replaced
		// one.
		s.cleanupPropertyPhoto(ctx, key)
		return domain.Property{}, err
	}
	if oldKey != nil {
		s.cleanupPropertyPhoto(ctx, *oldKey)
	}
	s.publishChanged(ctx, actor, propertyID, true)

	// The response carries the actor's personal pin (the cutover #1307) —
	// the photo write did not touch it, the reader's row is the truth.
	s.overlayActorPinDegrading(ctx, actor, &property)

	return property, nil
}

// DeletePropertyPhoto clears the photo columns; clearing an absent photo is
// ErrPhotoNotFound — the transport's 404. The object removal stays
// best-effort after the commit.
func (s *PropertyService) DeletePropertyPhoto(ctx context.Context, actor, propertyID uuid.UUID) (domain.Property, error) {
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return domain.Property{}, fmt.Errorf("resolve role: %w", err)
	}
	if role == sharedpolicy.RoleNone {
		return domain.Property{}, ErrNotFound
	}
	if !sharedpolicy.CanEdit(role) {
		return domain.Property{}, ErrForbidden
	}

	var (
		property domain.Property
		oldKey   *string
	)
	err = s.runInTx(ctx, func(stores *txStores) error {
		var err error
		property, err = stores.repo.GetByIDForUpdate(ctx, propertyID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("get property: %w", err)
		}

		if property.Status == domain.PropertyStatusArchived {
			return ErrArchivedProperty
		}

		if property.PhotoKey == nil {
			return ErrPhotoNotFound
		}
		oldKey = property.PhotoKey

		property, err = stores.repo.SetPropertyPhoto(ctx, propertyID, property.OwnerID, nil, nil)
		if err != nil {
			return fmt.Errorf("set property photo: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &actor,
			ActorRole:  sharedpolicy.AuditActorRole(role),
			Action:     auditdomain.ActionPropertyPhotoDeleted,
			EntityType: auditdomain.EntityPropertyPhoto,
			EntityID:   &propertyID,
			Context:    map[string]any{"property_id": propertyID},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(role), historydomain.PropertyPhotosChanged(propertyID, property.Name, false)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.Property{}, err
	}
	if oldKey != nil {
		s.cleanupPropertyPhoto(ctx, *oldKey)
	}
	s.publishChanged(ctx, actor, propertyID, true)

	// The response carries the actor's personal pin (the cutover #1307) —
	// the photo write did not touch it, the reader's row is the truth.
	s.overlayActorPinDegrading(ctx, actor, &property)

	return property, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// accessFailure maps a resolved role without the capability onto the error
// vocabulary: the suspended membership is the one distinguishable signal,
// everything else stays the privacy 404.
func (s *PropertyService) accessFailure(role sharedpolicy.Role) error {
	if role == sharedpolicy.RoleSuspended {
		return ErrAccessSuspended
	}
	return ErrNotFound
}

// auditFieldsKey is the audit context key listing the mutated fields.
const auditFieldsKey = "fields"

func sanitizeError(err error) string {
	return sanitize.Error(err)
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

// propertyUpdateHistoryEntry composes the action journal entry for an edit
// from the old/new property states (ADR 0061 §4): the unified change row
// (решение владельца 12.10, карта #1324) — «Объект изменён: [новое имя]»
// with one detail line per changed field; the former granular ids
// (renamed/address_changed/description_changed/attributes_changed) are gone.
// The status flip is not part of this composition — UpdateProperty journals
// it as the separate maintenance row (D2, карта #1324), so a status-only
// edit yields no entry here (ok=false) and a mixed PATCH yields this row
// plus the maintenance one. Equal attribute maps diff to nothing. The RU
// labels come from the calling side's catalogs: the field names here, the
// attribute labels from the generated catalog (attributeChanges), the type
// labels from PropertyTypeLabel.
func propertyUpdateHistoryEntry(propertyID uuid.UUID, before, after domain.Property) (historydomain.Entry, bool) {
	changes := make([]historydomain.AttributeChange, 0, 5)
	if before.Name != after.Name {
		changes = append(changes, historydomain.AttributeChange{
			Key: "name", Label: "Название", Old: before.Name, New: after.Name,
		})
	}
	if before.Address != after.Address {
		changes = append(changes, historydomain.AttributeChange{Key: "address", Label: "Адрес", Old: before.Address, New: after.Address})
	}
	if before.Description != after.Description {
		changes = append(changes, historydomain.AttributeChange{
			Key: "description", Label: "Описание", Old: before.Description, New: after.Description,
		})
	}
	if before.Type != after.Type {
		changes = append(changes, historydomain.AttributeChange{
			Key:   "type",
			Label: "Тип",
			Old:   domain.PropertyTypeLabel(before.Type),
			New:   domain.PropertyTypeLabel(after.Type),
		})
	}
	changes = append(changes, attributeChanges(before.Attributes, after.Attributes, after.Type)...)
	if len(changes) == 0 {
		return historydomain.Entry{}, false
	}
	return historydomain.PropertyUpdated(propertyID, after.Name, changes), true
}

// attributeChanges diffs two attribute maps over the union of their keys;
// nil folds to an empty map so a cleared attribute set reads as removals.
// The RU label comes from the generated catalog by the property's type —
// the detail lines of the new row template read it (history cannot import
// the catalog: направление зависимости properties → history). A PATCH that
// changes the type and the attributes together labels by the NEW type —
// the same type applyPropertyUpdate validated the values against.
func attributeChanges(before, after domain.Attributes, propType domain.PropertyType) []historydomain.AttributeChange {
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
		changes = append(changes, historydomain.AttributeChange{
			Key:   key,
			Label: domain.AttributeFieldLabel(propType, key),
			Old:   oldVal,
			New:   newVal,
		})
	}
	for key, oldVal := range before {
		if _, exists := after[key]; !exists {
			changes = append(changes, historydomain.AttributeChange{
				Key:   key,
				Label: domain.AttributeFieldLabel(propType, key),
				Old:   oldVal,
			})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Key < changes[j].Key })
	return changes
}
