package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
)

// ParticipantScopeProperty is one property of the reading actor's participant
// scope (issue #693): a non-archived property the actor owns or manages as an
// active full_access member. The scope is the authorization: every aggregate
// the participant read model returns is built exclusively from these rows, so
// a full_access member of one object sees only the legs they manage and never
// the rest of the owner's portfolio.
type ParticipantScopeProperty struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	Title   string
}

// ParticipantMembershipRow is a raw membership row inside the scope.
type ParticipantMembershipRow struct {
	PropertyID uuid.UUID
	UserID     uuid.UUID
	Role       domain.Role
	Status     domain.MemberStatus
}

// ParticipantInvitationRow is a raw pending invitation row inside the scope.
type ParticipantInvitationRow struct {
	PropertyID uuid.UUID
	Email      string // Normalized (lowercase) invitee email; PII (ADR 0020).
	Role       domain.Role
}

// ParticipantReadModel is the read port of the «Участник (владельца)»
// aggregate (issue #693): an overlay over property_members ∪
// property_member_invitations with no table of its own (chart decision of map
// #692). Implementations scope every query to the actor's manage scope.
type ParticipantReadModel interface {
	ListScopeProperties(ctx context.Context, actorID uuid.UUID) ([]ParticipantScopeProperty, error)
	// ListMembershipsByProperties returns all memberships (any status) on the
	// given properties. An empty id list returns no rows.
	ListMembershipsByProperties(ctx context.Context, propertyIDs []uuid.UUID) ([]ParticipantMembershipRow, error)
	// ListInvitationsByProperties returns all pending invitations on the given
	// properties. An empty id list returns no rows.
	ListInvitationsByProperties(ctx context.Context, propertyIDs []uuid.UUID) ([]ParticipantInvitationRow, error)
}

// AccessiblePropertiesCounter reports the number of active shared memberships
// a user holds — the «объекты пользователей» count of the participants hub
// summary (issue #693). Implemented by the membership repository's
// CountActiveByUser (the recipient tariff-slot counter, issue #163).
type AccessiblePropertiesCounter interface {
	CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

// ParticipantProperty is one leg of a participant's access: the property with
// its per-object role and lifecycle status (active/suspended/pending).
type ParticipantProperty struct {
	PropertyID uuid.UUID
	Title      string
	Role       domain.Role
	Status     domain.ParticipantEntryStatus
}

// Participant is the «Участник (владельца)» read model (issue #693): a person
// — a registered user or a pending email — with every access leg they hold on
// the reading actor's scoped properties, folded into the aggregate status.
// The property owner themselves is never a participant.
type Participant struct {
	// UserID is the registered user's id; zero for a pending email.
	UserID uuid.UUID
	// Email is the pending invitee address, or the registered user's email
	// resolved for display (empty when the user has none).
	Email       string
	DisplayName string
	// AggregateStatus and AccessibleCount are the aggregate badge («доступ ко
	// всем объектам» / «доступно N объектов» / «превышен лимит объектов»).
	AggregateStatus domain.ParticipantAggregateStatus
	AccessibleCount int
	// Properties are the access legs sorted by title.
	Properties []ParticipantProperty
}

// ParticipantsSummary carries the participants hub counters (issue #693): the
// number of the actor's participants and the number of other people's objects
// the actor has access to.
type ParticipantsSummary struct {
	ParticipantsCount         int
	AccessiblePropertiesCount int
}

// ParticipantsManager is the use-case interface the HTTP adapters consume
// (ADR 0035 func-backed test doubles); *ParticipantService implements it.
type ParticipantsManager interface {
	ListParticipants(ctx context.Context, actor uuid.UUID) ([]Participant, error)
	GetParticipant(ctx context.Context, actor uuid.UUID, participantID string) (Participant, error)
	Summary(ctx context.Context, actor uuid.UUID) (ParticipantsSummary, error)
}

// ParticipantService builds the owner's participant read model (issue #693).
// It is read-only: membership mutations stay on AccessService.
type ParticipantService struct {
	read       ParticipantReadModel
	users      UserLookup
	emails     UserEmailResolver
	accessible AccessiblePropertiesCounter
	logger     *slog.Logger
}

// NewParticipantService creates a ParticipantService. Emails may be nil to
// skip the registered users' email resolution; accessible may be nil to skip
// the hub summary's shared-objects count.
func NewParticipantService(
	read ParticipantReadModel,
	users UserLookup,
	emails UserEmailResolver,
	accessible AccessiblePropertiesCounter,
	logger *slog.Logger,
) *ParticipantService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ParticipantService{read: read, users: users, emails: emails, accessible: accessible, logger: logger}
}

// ListParticipants returns every person holding (or offered, when pending)
// access to the actor's scoped properties, sorted by display name with the
// pending emails last. The actor themself never appears: ownership is not a
// membership, and the scope SQL only admits memberships of other users.
func (s *ParticipantService) ListParticipants(ctx context.Context, actor uuid.UUID) ([]Participant, error) {
	return s.buildParticipants(ctx, actor, "")
}

// GetParticipant returns one person's aggregate by identifier. The identifier
// is a registered user's uuid or an invitee email (case-insensitive); both
// resolve to the same aggregate once the invitee registers. Everything not in
// the actor's scope is the privacy-preserving ErrParticipantNotFound.
func (s *ParticipantService) GetParticipant(ctx context.Context, actor uuid.UUID, participantID string) (Participant, error) {
	id := strings.TrimSpace(participantID)
	if _, err := uuid.Parse(id); err != nil {
		email, emailErr := domain.NormalizeEmail(id)
		if emailErr != nil {
			return Participant{}, domain.ErrParticipantNotFound
		}
		id = email
	}
	participants, err := s.buildParticipants(ctx, actor, id)
	if err != nil {
		return Participant{}, err
	}
	if len(participants) == 0 {
		return Participant{}, domain.ErrParticipantNotFound
	}
	return participants[0], nil
}

// Summary returns the participants hub counters (issue #693).
func (s *ParticipantService) Summary(ctx context.Context, actor uuid.UUID) (ParticipantsSummary, error) {
	participants, err := s.buildParticipants(ctx, actor, "")
	if err != nil {
		return ParticipantsSummary{}, err
	}
	summary := ParticipantsSummary{ParticipantsCount: len(participants)}
	if s.accessible != nil {
		count, err := s.accessible.CountActiveByUser(ctx, actor)
		if err != nil {
			return ParticipantsSummary{}, fmt.Errorf("count accessible properties: %w", err)
		}
		summary.AccessiblePropertiesCount = count
	}
	return summary, nil
}

// participantBuilder accumulates one person's legs while the raw scope rows
// are collated.
type participantBuilder struct {
	participant Participant
	seen        map[uuid.UUID]domain.ParticipantEntry
	legs        []domain.ParticipantEntry
}

// add appends a leg unless the property already has an equal-or-stronger one:
// membership rows collapse to one leg per property with active winning over
// suspended (the partial unique index tolerates both rows in the DB), and a
// pending invitation never overwrites a membership.
func (b *participantBuilder) add(entry domain.ParticipantEntry) {
	if existing, ok := b.seen[entry.PropertyID]; ok {
		if existing.Status == domain.ParticipantEntryActive {
			return
		}
		for i, leg := range b.legs {
			if leg.PropertyID == entry.PropertyID {
				b.legs[i] = entry
				break
			}
		}
		b.seen[entry.PropertyID] = entry
		return
	}
	b.seen[entry.PropertyID] = entry
	b.legs = append(b.legs, entry)
}

// buildParticipants assembles the aggregates from the scope's raw rows. When
// match is non-empty it selects exactly one person (by user uuid string or
// normalized email) instead of the full list.
func (s *ParticipantService) buildParticipants(ctx context.Context, actor uuid.UUID, match string) ([]Participant, error) {
	scope, err := s.read.ListScopeProperties(ctx, actor)
	if err != nil {
		return nil, fmt.Errorf("list scope properties: %w", err)
	}
	if len(scope) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(scope))
	titles := make(map[uuid.UUID]string, len(scope))
	for _, p := range scope {
		ids = append(ids, p.ID)
		titles[p.ID] = p.Title
	}

	memberships, err := s.read.ListMembershipsByProperties(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list scope memberships: %w", err)
	}
	invitations, err := s.read.ListInvitationsByProperties(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list scope invitations: %w", err)
	}

	byUser, byEmail := collateParticipants(memberships, invitations)
	// The reading actor themself never appears in their participant list:
	// ownership is not a membership, and a full_access manager reading the
	// list is not their own participant (issue #693).
	delete(byUser, actor)
	s.resolveDisplayData(ctx, byUser, byEmail)
	return assembleParticipants(byUser, byEmail, titles, len(scope), match), nil
}

// collateParticipants folds the raw membership and invitation rows into
// per-person builders keyed by user id (registered) or normalized email
// (pending).
func collateParticipants(
	memberships []ParticipantMembershipRow, invitations []ParticipantInvitationRow,
) (byUser map[uuid.UUID]*participantBuilder, byEmail map[string]*participantBuilder) {
	byUser = make(map[uuid.UUID]*participantBuilder)
	byEmail = make(map[string]*participantBuilder)
	for _, m := range memberships {
		b := participantBucket(byUser, byEmail, m.UserID, "")
		b.add(domain.ParticipantEntry{
			PropertyID: m.PropertyID,
			Role:       m.Role,
			Status:     domain.ParticipantEntryStatus(m.Status.String()),
		})
	}
	for _, inv := range invitations {
		email := strings.ToLower(inv.Email)
		b := participantBucket(byUser, byEmail, uuid.UUID{}, email)
		b.add(domain.ParticipantEntry{
			PropertyID: inv.PropertyID,
			Role:       inv.Role,
			Status:     domain.ParticipantEntryPending,
		})
	}
	return byUser, byEmail
}

// participantBucket returns (creating on demand) the builder of a person:
// by user id when the email is empty, by normalized email otherwise.
func participantBucket(
	byUser map[uuid.UUID]*participantBuilder, byEmail map[string]*participantBuilder,
	userID uuid.UUID, email string,
) *participantBuilder {
	if email == "" {
		if b, ok := byUser[userID]; ok {
			return b
		}
		b := &participantBuilder{seen: make(map[uuid.UUID]domain.ParticipantEntry)}
		b.participant.UserID = userID
		byUser[userID] = b
		return b
	}
	if b, ok := byEmail[email]; ok {
		return b
	}
	b := &participantBuilder{seen: make(map[uuid.UUID]domain.ParticipantEntry)}
	b.participant.Email = email
	byEmail[email] = b
	return b
}

// resolveDisplayData fills the registered users' display names and emails. A
// failed user lookup drops the person with a warning (same policy as
// ListMembers — the row is broken without its user); a failed email lookup
// just leaves the email empty. A leftover pending invitation whose email
// already belongs to a registered user collapses into that user's aggregate
// (normally invitations activate at registration, so this only bridges a
// race); legs on properties the user already holds lose to the membership.
func (s *ParticipantService) resolveDisplayData(
	ctx context.Context, byUser map[uuid.UUID]*participantBuilder, byEmail map[string]*participantBuilder,
) {
	for id, b := range byUser {
		u, err := s.users.GetByID(ctx, id)
		if err != nil {
			s.logger.WarnContext(ctx, "access: participant user lookup failed",
				slog.String(auditKeyUserID, id.String()), slog.String("error", err.Error()))
			delete(byUser, id)
			continue
		}
		b.participant.DisplayName = displayName(u)
		if s.emails == nil {
			continue
		}
		email, err := s.emails.GetEmail(ctx, id)
		if err != nil {
			s.logger.WarnContext(ctx, "access: participant email lookup failed",
				slog.String(auditKeyUserID, id.String()), slog.String("error", err.Error()))
			continue
		}
		b.participant.Email = email
		key := strings.ToLower(email)
		if pending, ok := byEmail[key]; ok {
			for _, leg := range pending.legs {
				b.add(leg)
			}
			delete(byEmail, key)
		}
	}
}

// assembleParticipants computes the aggregate badge per person, builds the
// title-annotated property legs, applies the single-person match filter and
// sorts the result. Byte-wise display-name order (named people first, pending
// emails last) — the client re-sorts for its «Имя» chip.
func assembleParticipants(
	byUser map[uuid.UUID]*participantBuilder, byEmail map[string]*participantBuilder,
	titles map[uuid.UUID]string, scopePropertyCount int, match string,
) []Participant {
	matchSelect := func(p Participant) bool {
		if match == "" {
			return true
		}
		if p.UserID != (uuid.UUID{}) {
			return p.UserID.String() == match || strings.EqualFold(p.Email, match)
		}
		return strings.EqualFold(p.Email, match)
	}
	out := make([]Participant, 0, len(byUser)+len(byEmail))
	for _, b := range byUser {
		if matchSelect(b.participant) {
			out = append(out, b.finish(titles, scopePropertyCount))
		}
	}
	for _, b := range byEmail {
		if matchSelect(b.participant) {
			out = append(out, b.finish(titles, scopePropertyCount))
		}
	}
	slices.SortFunc(out, func(a, c Participant) int {
		ag, an := participantSortKey(a)
		cg, cn := participantSortKey(c)
		if ag != cg {
			return ag - cg
		}
		if an != cn {
			return strings.Compare(an, cn)
		}
		return strings.Compare(participantTieBreak(a), participantTieBreak(c))
	})
	return out
}

// finish computes the aggregate badge and the title-annotated, title-ordered
// property legs of one person's aggregate.
func (b *participantBuilder) finish(titles map[uuid.UUID]string, scopePropertyCount int) Participant {
	b.participant.AggregateStatus, b.participant.AccessibleCount = domain.ComputeParticipantAggregate(b.legs, scopePropertyCount)
	legs := slices.Clone(b.legs)
	slices.SortFunc(legs, func(a, c domain.ParticipantEntry) int {
		return strings.Compare(titles[a.PropertyID], titles[c.PropertyID])
	})
	b.participant.Properties = make([]ParticipantProperty, 0, len(legs))
	for _, leg := range legs {
		b.participant.Properties = append(b.participant.Properties, ParticipantProperty{
			PropertyID: leg.PropertyID,
			Title:      titles[leg.PropertyID],
			Role:       leg.Role,
			Status:     leg.Status,
		})
	}
	return b.participant
}

// participantSortKey orders the list: named people first (display name,
// case-insensitive), pending email rows after (by email).
func participantSortKey(p Participant) (group int, key string) {
	if p.DisplayName != "" {
		return 0, strings.ToLower(p.DisplayName)
	}
	return 1, strings.ToLower(p.Email)
}

func participantTieBreak(p Participant) string {
	if p.UserID != (uuid.UUID{}) {
		return p.UserID.String()
	}
	return p.Email
}
