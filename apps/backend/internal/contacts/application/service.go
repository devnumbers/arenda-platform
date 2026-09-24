package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// CreateContactCommand is the create payload of a contact card. Every text
// field but the first name is optional; the phone travels in any accepted
// Russian spelling and is stored normalized.
type CreateContactCommand struct {
	// PropertyID optionally binds the card to a property; nil creates it in
	// the actor's own book («без объекта»).
	PropertyID        *uuid.UUID
	FirstName         string
	LastName          string
	Patronymic        string
	Role              string
	Phone             string
	Email             string
	MessengerName     string
	MessengerUsername string
	Note              string
}

// PropertyIDUpdate is the PATCH resolution of the property binding: Value nil
// clears it («без объекта»), a uuid moves the card onto that property.
type PropertyIDUpdate struct {
	Value *uuid.UUID
}

// UpdateContactCommand is the partial-update payload: a nil field leaves the
// card unchanged (omit = no change).
type UpdateContactCommand struct {
	FirstName         *string
	LastName          *string
	Patronymic        *string
	Role              *string
	Phone             *string
	Email             *string
	MessengerName     *string
	MessengerUsername *string
	Note              *string
	// PropertyID is tri-state: a nil command field keeps the current
	// binding; a non-nil one applies the PropertyIDUpdate (move or clear).
	PropertyID *PropertyIDUpdate
}

// ContactService orchestrates the contact book use cases (ADR 0054): create,
// read, list with scope, search and sort, partial update and delete. Access
// follows the ADR 0028 matrix through the shared policy: a property-bound
// card is visible to the property's shared members (CanView) and editable by
// them (CanEdit), the owner rules their whole book, and a card without a
// property belongs to the owner's book alone. Every mutation records its
// audit entry inside the same transaction (ADR 0020), a property-bound one
// also its action journal row (ADR 0061); the context never carries the
// card's PII.
type ContactService struct {
	txStoreFactory
	policy sharedpolicy.Policy
}

// NewContactService builds the contact use case service over the shared
// transactional store factory and the authorization policy. A nil policy or
// property store is a wiring mistake: every property-scoped use case fails
// loudly on first use rather than silently narrowing the ADR 0028 matrix to
// owner-only.
func NewContactService(factory txStoreFactory, policy sharedpolicy.Policy) *ContactService {
	return &ContactService{txStoreFactory: factory, policy: policy}
}

// CreateContact creates a card. A card with a property lands in the property
// owner's book — a Full-Access member creates it on the owner's behalf; the
// gate is the property's edit capability. A card without a property is
// created in the actor's own book: there is no other authorization anchor,
// and a stranger has no book of someone else's to write into.
func (s *ContactService) CreateContact(
	ctx context.Context, actor uuid.UUID, cmd CreateContactCommand,
) (domain.Contact, error) {
	draft, err := newContactDraft(cmd)
	if err != nil {
		return domain.Contact{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return domain.Contact{}, fmt.Errorf("mint contact id: %w", err)
	}
	draft.ID = id

	role := sharedpolicy.RoleOwner
	auditCtx := map[string]any{}
	if cmd.PropertyID != nil {
		role, err = s.gate(ctx, actor, *cmd.PropertyID, sharedpolicy.CanEdit)
		if err != nil {
			return domain.Contact{}, err
		}
		ref, err := s.properties.Get(ctx, *cmd.PropertyID)
		if err != nil {
			return domain.Contact{}, err
		}
		draft.OwnerID = ref.OwnerID
		draft.PropertyID = cmd.PropertyID
		auditCtx["property_id"] = *cmd.PropertyID
	} else {
		draft.OwnerID = actor
	}

	var created domain.Contact
	err = s.runInTx(ctx, func(stores *txStores) error {
		created, err = stores.contacts.Create(ctx, draft)
		if err != nil {
			return fmt.Errorf("create contact: %w", err)
		}
		if err := recordContactAudit(ctx, stores, actor, role, auditdomain.ActionContactCreated, draft.ID, auditCtx); err != nil {
			return err
		}
		// The journal is the object's feed: a card created without a
		// property writes no row (ADR 0061 — the schema anchors every row
		// to a property).
		if draft.PropertyID != nil {
			if err := historyapp.RecordScoped(ctx, stores.history, *draft.PropertyID, actor,
				sharedpolicy.HistoryActorRole(role),
				historydomain.ContactCreated(draft.ID, draft.FullName())); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Contact{}, err
	}
	return created, nil
}

// GetContact returns one card. The unscoped by-id load authorizes from the
// card's own binding: a property-bound card passes the view gate, an unbound
// one is the owner's alone — everyone else gets the privacy-preserving 404.
func (s *ContactService) GetContact(ctx context.Context, actor, id uuid.UUID) (domain.Contact, error) {
	contact, err := s.contacts.GetByID(ctx, id)
	if err != nil {
		return domain.Contact{}, err
	}
	if _, err := s.authorize(ctx, actor, contact, sharedpolicy.CanView); err != nil {
		return domain.Contact{}, err
	}
	return contact, nil
}

// prepareListQuery validates the listing request in place: the known
// scope/sort/order keys, the page-size vocabulary and the continuation
// cursor, decoded into the After keyset key (ticket #600). The empty
// sort/order normalize to the defaults (name/asc) before the store — the
// SQL's keyset predicate and ORDER BY branch on the literal keys, so the
// window and the order must agree.
func prepareListQuery(q *ListQuery) error {
	switch q.Scope {
	case ListScopeProperty, ListScopeAll, ListScopeWithoutProperty:
	default:
		return ErrInvalidInput
	}
	switch q.Sort {
	case "", ListSortName, ListSortProperty:
	default:
		return ErrInvalidInput
	}
	switch q.Order {
	case "", ListOrderAsc, ListOrderDesc:
	default:
		return ErrInvalidInput
	}
	if q.Sort == "" {
		q.Sort = ListSortName
	}
	if q.Order == "" {
		q.Order = ListOrderAsc
	}
	if q.Limit == 0 {
		q.Limit = DefaultContactsPageSize
	}
	if q.Limit < 1 || q.Limit > MaxContactsPageSize {
		return ErrInvalidInput
	}
	if q.Cursor == "" {
		return nil
	}
	return bindCursor(q)
}

// bindCursor decodes the echoed cursor into the After keyset key and binds
// it to the walk it came from: a cursor echoed under a different sort/order
// would misread every key component — the contract's 400, not a silent
// wrong window.
func bindCursor(q *ListQuery) error {
	after, cursorSort, cursorOrder, err := DecodeContactCursor(q.Cursor)
	if err != nil {
		return err
	}
	if cursorSort != q.Sort || cursorOrder != q.Order {
		return fmt.Errorf("cursor sort/order %q/%q does not match query %q/%q: %w",
			cursorSort, cursorOrder, q.Sort, q.Order, ErrInvalidInput)
	}
	q.After = &after
	return nil
}

// ListContacts lists one page of the visible book. The flat book scope reads
// the merged visibility — the actor's own cards plus the property-bound
// cards of the properties the actor can view; the store enforces that
// predicate, visibility being driven by the binding (ADR 0054). The property
// scope gates the actor's view capability on the property; the unbound scope
// reads the actor's own cards alone. The window is the listing's keyset walk
// (ticket #600): the page resumes strictly after the cursor's key and
// answers with the last row's continuation once it came back full — a short
// page has reached the end of the matches.
func (s *ContactService) ListContacts(
	ctx context.Context, actor uuid.UUID, q ListQuery,
) (ContactBookPage, error) {
	if q.Scope == ListScopeProperty {
		if _, err := s.gate(ctx, actor, q.PropertyID, sharedpolicy.CanView); err != nil {
			return ContactBookPage{}, err
		}
	}
	if err := prepareListQuery(&q); err != nil {
		return ContactBookPage{}, err
	}
	contacts, err := s.contacts.List(ctx, actor, q)
	if err != nil {
		return ContactBookPage{}, fmt.Errorf("list contacts: %w", err)
	}
	nextCursor := ""
	if len(contacts) == int(q.Limit) && len(contacts) > 0 {
		nextCursor = EncodeContactCursor(contacts[len(contacts)-1].CursorKey(), q.Sort, q.Order)
	}
	return ContactBookPage{Items: contacts, NextCursor: nextCursor}, nil
}

// UpdateContact applies a diff-patch to the card: omitted fields are left
// unchanged. Moving the card onto another property gates the target's edit
// capability too — the source gate alone must not authorize a cross-property
// move. The update runs scoped by (id, owner_id), so a card deleted between
// the pre-transaction read and the write surfaces as ErrNotFound and takes
// the audit entry down with it.
func (s *ContactService) UpdateContact(
	ctx context.Context, actor, id uuid.UUID, cmd UpdateContactCommand,
) (domain.Contact, error) {
	contact, err := s.contacts.GetByID(ctx, id)
	if err != nil {
		return domain.Contact{}, err
	}
	role, err := s.authorize(ctx, actor, contact, sharedpolicy.CanEdit)
	if err != nil {
		return domain.Contact{}, err
	}

	auditCtx := map[string]any{}
	fields := updatedContactFields(cmd)
	if len(fields) > 0 {
		auditCtx["fields"] = fields
	}
	var moved bool
	if cmd.PropertyID != nil {
		target, didMove, err := s.rebindContact(ctx, actor, contact, *cmd.PropertyID, auditCtx)
		if err != nil {
			return domain.Contact{}, err
		}
		contact.PropertyID = target
		moved = didMove
	}
	before := contact
	oldFullName := contact.FullName()
	if err := applyUpdate(&contact, cmd); err != nil {
		return domain.Contact{}, err
	}
	if err := domain.Validate(contact); err != nil {
		return domain.Contact{}, err
	}

	var stored domain.Contact
	err = s.runInTx(ctx, func(stores *txStores) error {
		stored, err = stores.contacts.Update(ctx, contact)
		if err != nil {
			return err
		}
		if err := recordContactAudit(ctx, stores, actor, role, auditdomain.ActionContactUpdated, contact.ID, auditCtx); err != nil {
			return err
		}
		// The row hangs on the card's final binding and records a change,
		// not a no-op (ADR 0061 §3): a same-form PATCH without a move writes
		// no row; a move between properties journals on the destination; an
		// unbound card writes no row.
		if stored.PropertyID != nil && (moved || contactChanged(before, stored)) {
			if err := historyapp.RecordScoped(ctx, stores.history, *stored.PropertyID, actor,
				sharedpolicy.HistoryActorRole(role),
				historydomain.ContactUpdated(contact.ID, oldFullName, stored.FullName())); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Contact{}, err
	}
	return stored, nil
}

// rebindContact rebinds the card onto the requested property and records the
// move's audit facts into auditCtx: the cleared binding or the destination
// property id. The returned target rebinds the card; moved says whether the
// binding actually changed.
func (s *ContactService) rebindContact(
	ctx context.Context, actor uuid.UUID, contact domain.Contact, upd PropertyIDUpdate, auditCtx map[string]any,
) (*uuid.UUID, bool, error) {
	target, moved, err := s.resolvePropertyBinding(ctx, actor, contact, upd)
	if err != nil {
		return nil, false, err
	}
	if moved {
		if target == nil {
			auditCtx["property_cleared"] = true
		} else {
			auditCtx["to_property_id"] = *target
		}
	}
	return target, moved, nil
}

// DeleteContact removes the card from the book, scoped by (id, owner_id) like
// the update.
func (s *ContactService) DeleteContact(ctx context.Context, actor, id uuid.UUID) error {
	contact, err := s.contacts.GetByID(ctx, id)
	if err != nil {
		return err
	}
	role, err := s.authorize(ctx, actor, contact, sharedpolicy.CanEdit)
	if err != nil {
		return err
	}
	auditCtx := map[string]any{}
	if contact.PropertyID != nil {
		auditCtx["property_id"] = *contact.PropertyID
	}
	return s.runInTx(ctx, func(stores *txStores) error {
		if err := stores.contacts.Delete(ctx, contact.ID, contact.OwnerID); err != nil {
			return err
		}
		if err := recordContactAudit(ctx, stores, actor, role, auditdomain.ActionContactDeleted, contact.ID, auditCtx); err != nil {
			return err
		}
		if contact.PropertyID != nil {
			if err := historyapp.RecordScoped(ctx, stores.history, *contact.PropertyID, actor,
				sharedpolicy.HistoryActorRole(role),
				historydomain.ContactDeleted(contact.ID, contact.FullName())); err != nil {
				return err
			}
		}
		return nil
	})
}

// gate applies the ADR 0028 capability gate over the policy and returns the
// resolved role for the audit trail: none and suspended stay
// privacy-preserving (ErrNotFound — the existence of the data is never
// revealed), a role without the capability is a straight ErrForbidden. A nil
// policy is the wiring mistake NewContactService documents.
func (s *ContactService) gate(
	ctx context.Context, actor, propertyID uuid.UUID, can func(sharedpolicy.Role) bool,
) (sharedpolicy.Role, error) {
	if s.policy == nil {
		return "", errors.New("contacts: authorization policy is not configured")
	}
	role, err := s.policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	switch sharedpolicy.GateFor(role, can) {
	case sharedpolicy.GateAllow:
		return role, nil
	case sharedpolicy.GateForbidden:
		return "", ErrForbidden
	default: // GateNone, GateSuspended.
		return "", ErrNotFound
	}
}

// authorize gates a loaded card from its own binding and returns the resolved
// role. A property-bound card gates on its property; an unbound one belongs
// to the owner's book alone — anyone else gets the privacy 404, never a
// forbidden that would reveal the card's existence.
func (s *ContactService) authorize(
	ctx context.Context, actor uuid.UUID, contact domain.Contact, can func(sharedpolicy.Role) bool,
) (sharedpolicy.Role, error) {
	if contact.PropertyID == nil {
		if contact.OwnerID != actor {
			return "", ErrNotFound
		}
		return sharedpolicy.RoleOwner, nil
	}
	return s.gate(ctx, actor, *contact.PropertyID, can)
}

// resolvePropertyBinding validates a patch's property move: the target needs
// the actor's edit capability (which also hides a nonexistent or foreign
// property behind the privacy 404). Clearing is always available to an actor
// who already passed the card's own edit gate. The book owner never changes:
// the card lives in one book, properties come and go.
func (s *ContactService) resolvePropertyBinding(
	ctx context.Context, actor uuid.UUID, contact domain.Contact, upd PropertyIDUpdate,
) (*uuid.UUID, bool, error) {
	switch {
	case upd.Value == nil:
		if contact.PropertyID == nil {
			return nil, false, nil
		}
		return nil, true, nil
	case contact.PropertyID != nil && *upd.Value == *contact.PropertyID:
		return contact.PropertyID, false, nil
	default:
		if _, err := s.gate(ctx, actor, *upd.Value, sharedpolicy.CanEdit); err != nil {
			return nil, false, err
		}
		return upd.Value, true, nil
	}
}

// newContactDraft builds the validated create draft: every text field is
// trimmed, whitespace-only optionals fold to "", the phone normalizes, and
// the whole draft passes the domain validator.
func newContactDraft(cmd CreateContactCommand) (domain.Contact, error) {
	draft := domain.Contact{
		FirstName:         strings.TrimSpace(cmd.FirstName),
		LastName:          strings.TrimSpace(cmd.LastName),
		Patronymic:        strings.TrimSpace(cmd.Patronymic),
		Role:              strings.TrimSpace(cmd.Role),
		Email:             strings.TrimSpace(cmd.Email),
		MessengerName:     strings.TrimSpace(cmd.MessengerName),
		MessengerUsername: strings.TrimSpace(cmd.MessengerUsername),
		Note:              strings.TrimSpace(cmd.Note),
	}
	phone, err := normalizePhoneField(cmd.Phone)
	if err != nil {
		return domain.Contact{}, err
	}
	draft.Phone = phone
	if err := domain.Validate(draft); err != nil {
		return domain.Contact{}, err
	}
	return draft, nil
}

// applyUpdate folds the command's diff into the card; only provided fields
// change, each normalized and validated as at creation by the caller's
// Validate pass. A malformed phone in the diff is the contract's invalid
// input — folding must never silently drop it.
func applyUpdate(contact *domain.Contact, cmd UpdateContactCommand) error {
	if cmd.FirstName != nil {
		contact.FirstName = strings.TrimSpace(*cmd.FirstName)
	}
	if cmd.LastName != nil {
		contact.LastName = strings.TrimSpace(*cmd.LastName)
	}
	if cmd.Patronymic != nil {
		contact.Patronymic = strings.TrimSpace(*cmd.Patronymic)
	}
	if cmd.Role != nil {
		contact.Role = strings.TrimSpace(*cmd.Role)
	}
	if cmd.Phone != nil {
		phone, err := normalizePhoneField(*cmd.Phone)
		if err != nil {
			return err
		}
		contact.Phone = phone
	}
	if cmd.Email != nil {
		contact.Email = strings.TrimSpace(*cmd.Email)
	}
	if cmd.MessengerName != nil {
		contact.MessengerName = strings.TrimSpace(*cmd.MessengerName)
	}
	if cmd.MessengerUsername != nil {
		contact.MessengerUsername = strings.TrimSpace(*cmd.MessengerUsername)
	}
	if cmd.Note != nil {
		contact.Note = strings.TrimSpace(*cmd.Note)
	}
	return nil
}

// contactChanged compares the card fields the journal row speaks about
// (ADR 0061 §4): the ФИО and the contact details. The property binding is
// not part of the row text — a binding-only change travels in UpdateContact's
// moved flag, never here.
func contactChanged(before, after domain.Contact) bool {
	return before.FullName() != after.FullName() ||
		before.Role != after.Role ||
		before.Phone != after.Phone ||
		before.Email != after.Email ||
		before.MessengerName != after.MessengerName ||
		before.MessengerUsername != after.MessengerUsername ||
		before.Note != after.Note
}

// normalizePhoneField folds an absent phone to "" and normalizes a present
// one; a malformed value is the contract's invalid input.
func normalizePhoneField(phone string) (string, error) {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return "", nil
	}
	normalized, err := domain.NormalizePhone(trimmed)
	if err != nil {
		return "", ErrInvalidInput
	}
	return normalized, nil
}

// updatedContactFields lists the field names a command changes; the audit
// context carries names only, never the card's values (the card is PII).
func updatedContactFields(cmd UpdateContactCommand) []string {
	fields := make([]string, 0, 10)
	if cmd.FirstName != nil {
		fields = append(fields, "first_name")
	}
	if cmd.LastName != nil {
		fields = append(fields, "last_name")
	}
	if cmd.Patronymic != nil {
		fields = append(fields, "patronymic")
	}
	if cmd.Role != nil {
		fields = append(fields, "role")
	}
	if cmd.Phone != nil {
		fields = append(fields, "phone")
	}
	if cmd.Email != nil {
		fields = append(fields, "email")
	}
	if cmd.MessengerName != nil {
		fields = append(fields, "messenger_name")
	}
	if cmd.MessengerUsername != nil {
		fields = append(fields, "messenger_username")
	}
	if cmd.Note != nil {
		fields = append(fields, "note")
	}
	if cmd.PropertyID != nil {
		fields = append(fields, "property_id")
	}
	return fields
}

// recordContactAudit writes the mutation's audit entry inside the transaction
// (fail-safe: an insert error rolls the mutation back, ADR 0020). The role is
// the one the gate resolved; the context carries whitelisted keys only.
func recordContactAudit(
	ctx context.Context, stores *txStores, actor uuid.UUID, role sharedpolicy.Role,
	action auditdomain.Action, contactID uuid.UUID, auditCtx map[string]any,
) error {
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(role),
		Action:     action,
		EntityType: auditdomain.EntityContact,
		EntityID:   &contactID,
		Context:    auditCtx,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}
