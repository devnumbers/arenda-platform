package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// AccessPropertyView is the display snapshot of the property an access event
// happened on: the shared cards' name and address line (решение владельца
// 19.09.2026, #745).
type AccessPropertyView struct {
	Name    string
	Address string
}

// AccessUserProfile is the display snapshot of a user an access event names:
// the display name (the access canon — the name, or the full phone when the
// profile has none, карта #1105) and the email the actor card shows (#745).
type AccessUserProfile struct {
	DisplayName string
	Email       string
}

// AccessEventViewSource resolves the display snapshots of the access
// events (#751). Consumer-declared (CODING_STANDARDS), answered over the
// owning tables: the property from properties, the user profile over the
// identity user repository.
type AccessEventViewSource interface {
	PropertyView(ctx context.Context, propertyID uuid.UUID) (AccessPropertyView, error)
	UserProfileView(ctx context.Context, userID uuid.UUID) (AccessUserProfile, error)
}

// AccessPublisher is the access events' publisher (#751) on the delivery
// pipeline (карта #734, #740): the transition hooks of the access context —
// an invitation's activation or an instant grant, a revoke, a slot pause or
// recovery, a member's self-exit, a member's role change — arrive here and
// become the Совместный доступ catalog rows (решение #737, типы №5–№10 и
// access_role_changed, #830) for their single addressee. The former direct
// lifecycle emails (issue #162, T6) are gone: same events, but the
// notification is now a stored feed row — written always, delivered over
// email and push per the category matrix (ADR 0058). The invite email to an
// unregistered address stays out of the pipeline by nature: until the invitee
// registers there is no recipient a row could belong to (№5's row is the
// activation's or the instant grant's, решение #737).
//
// The copy's {Имя} is the actor's display name at publication time; a system
// transition (the slot enforcement, no human initiator) renders the no-name
// variant — the catalog's own no-name pattern (the №10 example). The display
// snapshots are resolved at publication: the row is the moment's snapshot
// (EntityRef, #745), a renamed property or a later profile edit never
// rewrites stored rows.
type AccessPublisher struct {
	pipeline *Publisher
	views    AccessEventViewSource
}

// NewAccessPublisher builds the access events' publisher over the pipeline
// creation service and the view source.
func NewAccessPublisher(pipeline *Publisher, views AccessEventViewSource) *AccessPublisher {
	return &AccessPublisher{pipeline: pipeline, views: views}
}

// NotifyInvitationActivated publishes a pending invitation's activation at
// registration (решение #737, типы №5–№6): the inviter always learns the
// invitee accepted («Приглашение принято»); the invitee's own row depends on
// the landing state — «Приглашение в объект» when the access is active, the
// system «Доступ приостановлен» when the activation consumed no free slot
// (the invitation copy would promise an access that is hidden). ActivatedAt
// is the activation instant the paused row's dedup key stamps.
func (p *AccessPublisher) NotifyInvitationActivated(
	ctx context.Context,
	membershipID, propertyID, inviterID, inviteeID uuid.UUID,
	suspended bool,
	activatedAt time.Time,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	inviter, err := p.views.UserProfileView(ctx, inviterID)
	if err != nil {
		return fmt.Errorf("resolve inviter profile %s: %w", inviterID, err)
	}
	invitee, err := p.views.UserProfileView(ctx, inviteeID)
	if err != nil {
		return fmt.Errorf("resolve invitee profile %s: %w", inviteeID, err)
	}

	// №6 «Приглашение принято» to the inviter; the invitee is the actor.
	if err := p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventInvitationAccepted,
		DedupKey:     domain.DedupKey("invitation_accepted:" + membershipID.String()),
		Title:        "Приглашение принято",
		Body:         fmt.Sprintf("%s принял приглашение в объект «%s»", invitee.DisplayName, property.Name),
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, inviteeID, &invitee, membershipID),
		Recipients:   []uuid.UUID{inviterID},
		Actor:        inviteeID,
	}); err != nil {
		return err
	}

	if suspended {
		return p.publishPaused(ctx, membershipID, propertyID, property, inviteeID, uuid.Nil, nil, activatedAt)
	}

	// №5 «Приглашение в объект» to the invitee; the inviter is the actor.
	return p.publishInvitationRow(ctx, membershipID, propertyID, inviterID, inviteeID, property, inviter)
}

// NotifyMembershipGranted publishes the №5 «Приглашение в объект» row for the
// instant landing (issue #829): a registered user granted access right away —
// by the batch Invite/AddProperties or the single InviteByEmail — learns about
// it with the catalog's verbatim copy, the granter the actor. The same copy
// the registration activation uses: the access is active, so «Теперь объект
// доступен вам совместно» is true. No «Приглашение принято» exists here —
// there was no acceptance; a no-slot landing never arrives as granted (it
// speaks the system pause).
func (p *AccessPublisher) NotifyMembershipGranted(
	ctx context.Context,
	membershipID, propertyID, recipientID, actorID uuid.UUID,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	inviter, err := p.views.UserProfileView(ctx, actorID)
	if err != nil {
		return fmt.Errorf("resolve inviter profile %s: %w", actorID, err)
	}
	return p.publishInvitationRow(ctx, membershipID, propertyID, actorID, recipientID, property, inviter)
}

// NotifyRoleChanged publishes the «Роль изменена» row (карта #828, тикет
// #830, решение владельца 23.09): a manager changed the member's role, and
// the member learns it — the changer the actor, the recipient the member.
// The role names speak the #692 chart canon's display wording
// («Редактирование»/«Просмотр»); ChangedAt is the change instant (the
// membership row's updated_at) the dedup key stamps — every change is its
// own fact and notifies anew, a repeat publication of the same change
// inserts nothing.
func (p *AccessPublisher) NotifyRoleChanged(
	ctx context.Context,
	membershipID, propertyID, recipientID, actorID uuid.UUID,
	role string,
	changedAt time.Time,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	actor, err := p.views.UserProfileView(ctx, actorID)
	if err != nil {
		return fmt.Errorf("resolve actor profile %s: %w", actorID, err)
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType: domain.EventAccessRoleChanged,
		DedupKey:  domain.DedupKey("access_role_changed:" + membershipID.String() + ":" + unixDedupStamp(changedAt)),
		Title:     "Роль изменена",
		Body: fmt.Sprintf("%s изменил вашу роль в объекте «%s» на «%s»",
			actor.DisplayName, property.Name, roleChangedLabel(role)),
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, actorID, &actor, membershipID),
		Recipients:   []uuid.UUID{recipientID},
		Actor:        actorID,
	})
}

// roleChangedLabel renders the membership role in the display wording of the
// #692 chart canon (the frontend's shared/model/access labels): full_access
// reads «Редактирование», viewer «Просмотр». Anything else renders the raw
// role — a defensive default the two catalog roles never hit.
func roleChangedLabel(role string) string {
	switch role {
	case "full_access":
		return "Редактирование"
	case "viewer":
		return "Просмотр"
	default:
		return role
	}
}

// publishInvitationRow is the №5 row's shared body (the registration
// activation's active landing #751 and the instant landing #829): the catalog
// copy verbatim, the inviter naming the text and the actor card, the bare
// membership id keying the row — revoke → re-invite is a new membership,
// hence a new row.
func (p *AccessPublisher) publishInvitationRow(
	ctx context.Context,
	membershipID, propertyID, inviterID, recipientID uuid.UUID,
	property AccessPropertyView,
	inviter AccessUserProfile,
) error {
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventPropertyInvitation,
		DedupKey:     domain.DedupKey("property_invitation:" + membershipID.String()),
		Title:        "Приглашение в объект",
		Body:         fmt.Sprintf("%s пригласил вас в объект «%s». Теперь объект доступен вам совместно", inviter.DisplayName, property.Name),
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, inviterID, &inviter, membershipID),
		Recipients:   []uuid.UUID{recipientID},
		Actor:        inviterID,
	})
}

// NotifyMembershipSuspended publishes the «Доступ приостановлен» row (№737
// тип №8) to the paused member. ActorID uuid.Nil marks the system
// suspension (the slot enforcement) — the no-name copy; a human actor (a
// future manual suspend) names them. SuspendedAt stamps the dedup key: a
// membership can cycle paused → resumed → paused, and each pause is its own
// fact — the instant keeps every cycle's row while a repeat publication of
// the same transition inserts nothing.
func (p *AccessPublisher) NotifyMembershipSuspended(
	ctx context.Context,
	membershipID, propertyID, recipientID, actorID uuid.UUID,
	suspendedAt time.Time,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	_, actorRef, err := resolveActor(ctx, p.views, actorID)
	if err != nil {
		return err
	}
	return p.publishPaused(ctx, membershipID, propertyID, property, recipientID, actorID, actorRef, suspendedAt)
}

// NotifyMembershipResumed publishes the «Доступ восстановлен» row (решение
// #737, тип №9) to the recovered member. Today the recovery is always the
// slot coordinator's — the no-name copy; the actor shape stays for a future
// manual resume, as the catalog's copy presumes one.
func (p *AccessPublisher) NotifyMembershipResumed(
	ctx context.Context,
	membershipID, propertyID, recipientID, actorID uuid.UUID,
	resumedAt time.Time,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	actor, actorRef, err := resolveActor(ctx, p.views, actorID)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("Ваш доступ к объекту «%s» восстановлен", property.Name)
	if actorRef != nil {
		body = fmt.Sprintf("%s восстановил ваш доступ к объекту «%s»",
			actor.DisplayName, property.Name)
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventAccessResumed,
		DedupKey:     domain.DedupKey("access_resumed:" + membershipID.String() + ":" + unixDedupStamp(resumedAt)),
		Title:        "Доступ восстановлен",
		Body:         body,
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, actorID, actorRef, membershipID),
		Recipients:   []uuid.UUID{recipientID},
		Actor:        actorID,
	})
}

// NotifyAccessRevoked publishes the «Доступ отозван» row (решение #737, тип
// №7) to the removed member; the revoking owner or full member is the actor.
// The membership row is deleted by the transition, so the bare membership id
// keys the row — the id can never repeat.
func (p *AccessPublisher) NotifyAccessRevoked(
	ctx context.Context,
	membershipID, propertyID, recipientID, actorID uuid.UUID,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	actor, actorRef, err := humanActor(ctx, p.views, actorID)
	if err != nil {
		return err
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType: domain.EventAccessRevoked,
		DedupKey:  domain.DedupKey("access_revoked:" + membershipID.String()),
		Title:     "Доступ отозван",
		Body: fmt.Sprintf("%s отозвал ваш доступ к объекту «%s». Объект больше не отображается в вашей книге",
			actor.DisplayName, property.Name),
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, actorID, actorRef, membershipID),
		Recipients:   []uuid.UUID{recipientID},
		Actor:        actorID,
	})
}

// NotifyMemberLeft publishes the «Участник покинул объект» row (решение
// #737, тип №10) to the owner; the leaving member is the actor.
func (p *AccessPublisher) NotifyMemberLeft(
	ctx context.Context,
	membershipID, propertyID, ownerID, memberID uuid.UUID,
) error {
	property, err := p.views.PropertyView(ctx, propertyID)
	if err != nil {
		return fmt.Errorf("resolve property view %s: %w", propertyID, err)
	}
	actor, actorRef, err := humanActor(ctx, p.views, memberID)
	if err != nil {
		return err
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventMemberLeft,
		DedupKey:     domain.DedupKey("member_left:" + membershipID.String()),
		Title:        "Участник покинул объект",
		Body:         fmt.Sprintf("%s больше не имеет доступа к объекту «%s»", actor.DisplayName, property.Name),
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, memberID, actorRef, membershipID),
		Recipients:   []uuid.UUID{ownerID},
		Actor:        memberID,
	})
}

// publishPaused is the shared body of the two pause shapes (the
// activation-without-slot and the slot enforcement): the system copy unless
// a human actor names them.
func (p *AccessPublisher) publishPaused(
	ctx context.Context,
	membershipID, propertyID uuid.UUID,
	property AccessPropertyView,
	recipientID, actorID uuid.UUID,
	actorRef *AccessUserProfile,
	suspendedAt time.Time,
) error {
	body := fmt.Sprintf("Ваш доступ к объекту «%s» приостановлен. Данные объекта скрыты, пока доступ приостановлен",
		property.Name)
	if actorRef != nil {
		body = fmt.Sprintf("%s приостановил ваш доступ к объекту «%s». Данные объекта скрыты, пока доступ приостановлен",
			actorRef.DisplayName, property.Name)
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventAccessPaused,
		DedupKey:     domain.DedupKey("access_paused:" + membershipID.String() + ":" + unixDedupStamp(suspendedAt)),
		Title:        "Доступ приостановлен",
		Body:         body,
		ContextLabel: property.Name,
		Payload:      accessPayload(propertyID, property, actorID, actorRef, membershipID),
		Recipients:   []uuid.UUID{recipientID},
		Actor:        actorID,
	})
}

// resolveActor resolves the transition's actor display snapshot. ActorID
// uuid.Nil marks the system transition (the slot enforcement): no snapshot,
// the no-name copy. The pointer shares the resolved profile with the payload
// builder; the value keeps the copy's subject.
func resolveActor(
	ctx context.Context, views AccessEventViewSource, actorID uuid.UUID,
) (profile AccessUserProfile, ref *AccessUserProfile, err error) {
	if actorID == uuid.Nil {
		return AccessUserProfile{}, nil, nil
	}
	profile, err = views.UserProfileView(ctx, actorID)
	if err != nil {
		return AccessUserProfile{}, nil, fmt.Errorf("resolve actor profile %s: %w", actorID, err)
	}
	return profile, &profile, nil
}

// humanActor resolves the actor of the transitions whose copy has no no-name
// variant (the revoke, the self-exit — the actor is inherent): a system
// actor here is the caller's bug, not a rendering case.
func humanActor(
	ctx context.Context, views AccessEventViewSource, actorID uuid.UUID,
) (profile AccessUserProfile, ref *AccessUserProfile, err error) {
	if actorID == uuid.Nil {
		return AccessUserProfile{}, nil, fmt.Errorf("access event %s requires a human actor", actorID)
	}
	return resolveActor(ctx, views, actorID)
}

// accessPayload builds the access events' payload: the property card, the
// actor card when the transition has a human actor, and the membership id —
// the transition's entity (решение #737: payload membership_id).
func accessPayload(
	propertyID uuid.UUID,
	property AccessPropertyView,
	actorID uuid.UUID,
	actor *AccessUserProfile,
	membershipID uuid.UUID,
) domain.Payload {
	payload := domain.Payload{
		Property: &domain.EntityRef{
			ID:      propertyID,
			Name:    property.Name,
			Address: property.Address,
		},
		MembershipID: &membershipID,
	}
	if actor != nil && actorID != uuid.Nil {
		payload.Actor = &domain.EntityRef{
			ID:    actorID,
			Name:  actor.DisplayName,
			Email: actor.Email,
		}
	}
	return payload
}

// unixDedupStamp renders the paused/resumed and role-changed keys' instant
// half: the unix seconds of the transition.
func unixDedupStamp(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}
