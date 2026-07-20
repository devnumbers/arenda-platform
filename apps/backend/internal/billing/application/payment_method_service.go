package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// pendingCardBindingTTL is the lifetime of a T-Kassa AddCard bank-form
// session: the form expires after 2 days, so a placeholder older than that can
// never complete and is dropped by the sync without polling the provider.
const pendingCardBindingTTL = 48 * time.Hour

// PaymentMethodService manages the user's saved payment methods.
type PaymentMethodService struct {
	deps     paymentMethodServiceDeps
	checker  PaymentMethodInUseChecker
	provider CardProvider
}

// NewPaymentMethodService creates a PaymentMethodService.
func NewPaymentMethodService(deps paymentMethodServiceDeps, checker PaymentMethodInUseChecker, provider CardProvider) *PaymentMethodService {
	return &PaymentMethodService{deps: deps, checker: checker, provider: provider}
}

// AddPaymentMethod stores a new inactive payment method for the user.
// For the fake provider the method is created synchronously from the raw token.
// For T-Kassa a bank-form flow is initiated and the confirmation URL is returned.
func (s *PaymentMethodService) AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (AddPaymentMethodResponse, error) {
	if s.provider.Name() == domain.ProviderFake {
		pm, err := domain.NewPaymentMethod(
			userID,
			s.provider.Name(),
			req.ProviderToken,
			maskToken(req.ProviderToken),
			s.deps.clock.Now().UTC(),
		)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("create payment method: %w", err)
		}

		tx, err := s.deps.beginner.Begin(ctx)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("bind payment methods transaction: %w", err)
		}

		pm, err = txPaymentMethods.Create(ctx, pm)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("save payment method: %w", err)
		}

		if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPaymentMethodAdded,
			EntityType: auditdomain.EntityPaymentMethod,
			EntityID:   &pm.ID,
		}); err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("record audit: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("commit add payment method transaction: %w", err)
		}

		return AddPaymentMethodResponse{PaymentMethod: &pm}, nil
	}

	successURL, failURL := tkassaAddCardReturnURLs(s.deps.callbackBaseURL)
	result, err := s.provider.InitAddCard(ctx, InitAddCardRequest{
		UserID:          userID,
		CustomerKey:     userID.String(),
		CheckType:       CardCheckType3DSHold,
		SuccessURL:      successURL,
		FailURL:         failURL,
		NotificationURL: tkassaNotificationURL(s.deps.callbackBaseURL),
	})
	if err != nil {
		return AddPaymentMethodResponse{}, sanitize.Wrap(err, "init add card")
	}

	// Note on T-Kassa error 510 ("card already bound to this CustomerKey"): it
	// cannot surface in this initiation flow, because the AddCard request
	// carries no card data — only the CustomerKey. A duplicate card instead
	// appears as the binding result or is already present in GetCardList, so no
	// idempotency handling is needed here.

	// Persist a pending binding placeholder keyed by the RequestKey so
	// SyncPaymentMethods can poll GetAddCardState when the AddCard webhook is
	// not delivered (for example on demo terminals without a notification URL).
	// The placeholder token carries the PendingCardBindingTokenPrefix so it is
	// explicitly distinguishable from real card rows that store a raw RebillId
	// (webhook, sync and RebillId-recovery paths). The placeholder is internal:
	// it is hidden from ListPaymentMethods and is dropped by the sync once the
	// binding completes or expires. Persistence is best-effort: when the row
	// cannot be saved the webhook path still binds the card, only the polling
	// fallback is lost.
	if s.provider.Name() == domain.ProviderTkassa && result.RequestKey != "" {
		pending, pmErr := domain.NewPaymentMethod(userID, s.provider.Name(), domain.PendingCardBindingToken(result.RequestKey), "", s.deps.clock.Now().UTC())
		if pmErr != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("create pending card binding: %w", pmErr)
		}
		if _, pmErr := s.deps.paymentMethods.Create(ctx, pending); pmErr != nil {
			s.deps.log.WarnContext(ctx, "failed to persist pending card binding; sync polling fallback disabled for this binding",
				slog.String("user_id", userID.String()),
				slog.String("error", sanitize.Error(pmErr)))
		}
	}

	return AddPaymentMethodResponse{ConfirmURL: result.PaymentURL}, nil
}

// SetActivePaymentMethod activates the given payment method for the user and
// makes it the active method for subscription renewals.
func (s *PaymentMethodService) SetActivePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind payment methods transaction: %w", err)
	}
	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	// A pending card-binding placeholder is internal sync state, not a usable
	// payment method: activating it must behave as if the row did not exist.
	pm, err := txPaymentMethods.GetByID(ctx, methodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentMethodNotFound
		}
		return fmt.Errorf("get payment method: %w", err)
	}
	if pm.PendingCardBindingRequestKey() != "" {
		return ErrPaymentMethodNotFound
	}

	if err := txPaymentMethods.SetActive(ctx, userID, methodID); err != nil {
		return fmt.Errorf("set active payment method: %w", err)
	}

	sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}
	sub.SetActivePaymentMethod(methodID)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription active payment method: %w", err)
	}

	if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPaymentMethodActivated,
		EntityType: auditdomain.EntityPaymentMethod,
		EntityID:   &methodID,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set active payment method transaction: %w", err)
	}
	return nil
}

// DeletePaymentMethod removes a payment method belonging to the user.
// Existence, ownership, the active-method guard and the in-use check all run
// inside a single transaction so they cannot race with concurrent updates.
// For T-Kassa the provider card is detached best-effort afterwards, so a
// provider failure cannot leave the local row in place.
func (s *PaymentMethodService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind payment methods transaction: %w", err)
	}

	txChecker, err := s.checker.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind in-use checker transaction: %w", err)
	}

	pm, err := txPaymentMethods.GetByID(ctx, methodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentMethodNotFound
		}
		return fmt.Errorf("get payment method: %w", err)
	}
	if pm.UserID != userID {
		return ErrPaymentMethodNotFound
	}
	if pm.IsActive {
		return ErrPaymentMethodInUse
	}

	inUse, err := txChecker.IsInUse(ctx, methodID)
	if err != nil {
		return fmt.Errorf("check payment method in use: %w", err)
	}
	if inUse {
		return ErrPaymentMethodInUse
	}

	if err := txPaymentMethods.Delete(ctx, userID, methodID); err != nil {
		return fmt.Errorf("delete payment method: %w", err)
	}

	if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionPaymentMethodDeleted,
		EntityType: auditdomain.EntityPaymentMethod,
		EntityID:   &methodID,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete payment method transaction: %w", err)
	}

	// The local row is gone; detach the card at the provider best-effort. A
	// provider failure must not fail the operation since the method is already deleted.
	if s.provider.Name() == domain.ProviderTkassa && pm.ProviderCardID != "" {
		if err := s.provider.RemoveCard(ctx, userID.String(), pm.ProviderCardID); err != nil {
			if errors.Is(err, ErrProviderCardNotFound) {
				s.deps.log.WarnContext(ctx, "provider card already removed; continuing local deletion",
					slog.String("payment_method_id", methodID.String()),
					slog.String("provider_card_id", maskCardID(pm.ProviderCardID)))
			} else {
				s.deps.log.ErrorContext(ctx, "failed to remove provider card after payment method deletion",
					slog.String("payment_method_id", methodID.String()),
					slog.String("provider_card_id", maskCardID(pm.ProviderCardID)),
					slog.String("error", sanitize.Error(err)))
			}
		}
	}

	return nil
}

// ListPaymentMethods returns all payment methods for the user. Pending
// card-binding placeholders are internal sync state and are not returned.
func (s *PaymentMethodService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	list, err := s.deps.paymentMethods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods: %w", err)
	}
	return filterPendingCardBindings(list), nil
}

// filterPendingCardBindings drops pending card-binding placeholders: they are
// internal sync state, not usable payment methods, and must not be shown to
// users or activated.
func filterPendingCardBindings(methods []domain.PaymentMethod) []domain.PaymentMethod {
	out := make([]domain.PaymentMethod, 0, len(methods))
	for _, m := range methods {
		if m.PendingCardBindingRequestKey() != "" {
			continue
		}
		out = append(out, m)
	}
	return out
}

// SyncPaymentMethods imports the cards bound at the provider into local payment
// methods and returns the user's up-to-date list. It is the self-healing path
// for card binding when the AddCard webhook is not delivered (for example on
// demo terminals without a configured notification URL).
//
// Before importing, every pending card-binding placeholder created by
// AddPaymentMethod is polled via GetAddCardState: a completed binding is
// imported from the binding state (CardID/RebillID) even when GetCardList does
// not report the card yet, a rejected binding or a RequestKey the provider no
// longer knows (error 502) is dropped as expired, and intermediate statuses
// keep the binding pending. A placeholder older than pendingCardBindingTTL is
// dropped without polling: the AddCard form has expired, so the binding can
// never complete. Only cards the provider reports as active with a
// recurrent token are imported, and the upsert is keyed by token hash, so
// repeated syncs and webhook deliveries converge on the same row. A freshly
// completed binding becomes the active method, mirroring the AddCard webhook
// flow; otherwise, when the user has no active method after the import, the
// freshest imported card is activated and linked to the subscription. A
// customer that does not exist at the provider yet (never paid or bound a
// card) is treated as "no cards": the sync returns the local list unchanged.
// The method is idempotent.
func (s *PaymentMethodService) SyncPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	existing, err := s.deps.paymentMethods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods before sync: %w", err)
	}

	// Poll pending card bindings by RequestKey. The provider calls happen
	// outside of any database transaction. completed holds the confirmed
	// bindings to import; stale holds placeholder rows to drop (completed,
	// rejected or expired bindings).
	var completed []CardBindingState
	var stale []uuid.UUID
	now := s.deps.clock.Now().UTC()
	for i := range existing {
		requestKey := existing[i].PendingCardBindingRequestKey()
		if requestKey == "" {
			continue
		}
		if existing[i].CreatedAt.Add(pendingCardBindingTTL).Before(now) {
			// The AddCard bank form has expired: the binding can never
			// complete, so drop the placeholder without polling the provider.
			stale = append(stale, existing[i].ID)
			continue
		}
		state, stateErr := s.provider.GetAddCardState(ctx, requestKey)
		if stateErr != nil {
			if code := providerErrorCode(stateErr); code != nil && *code == "502" {
				// The provider no longer knows the RequestKey: the binding
				// session expired without completing.
				stale = append(stale, existing[i].ID)
				continue
			}
			s.deps.log.WarnContext(ctx, "sync payment methods: failed to query add card state; keeping binding pending",
				slog.String("user_id", userID.String()),
				slog.String("payment_method_id", existing[i].ID.String()),
				slog.String("error", sanitize.Error(stateErr)))
			continue
		}
		switch state.Status {
		case CardBindingStatusCompleted:
			if state.RebillID == "" {
				s.deps.log.WarnContext(ctx, "sync payment methods: completed card binding has no rebill id; keeping binding pending",
					slog.String("user_id", userID.String()),
					slog.String("payment_method_id", existing[i].ID.String()))
				continue
			}
			completed = append(completed, state)
			stale = append(stale, existing[i].ID)
		case CardBindingStatusRejected:
			stale = append(stale, existing[i].ID)
		default:
			// NEW, FORM_SHOWED, 3DS_CHECKING, 3DS_CHECKED, AUTHORIZING,
			// AUTHORIZED: the binding is still in flight; keep it pending.
		}
	}

	cards, err := s.provider.GetCardList(ctx, userID.String())
	if err != nil {
		if errors.Is(err, ErrProviderCustomerNotFound) {
			return s.ListPaymentMethods(ctx, userID)
		}
		if errors.Is(err, ErrProviderTerminalNotFound) {
			s.deps.log.WarnContext(ctx, "sync payment methods: terminal not found at provider; returning local list",
				slog.String("user_id", userID.String()),
				slog.String("error", sanitize.Error(err)))
			return s.ListPaymentMethods(ctx, userID)
		}
		return nil, sanitize.Wrap(err, "get card list")
	}

	var importable []ProviderCard
	for _, card := range cards {
		if card.Status != ProviderCardStatusActive || card.RebillID == "" {
			continue
		}
		importable = append(importable, card)
	}

	// Cards confirmed via GetAddCardState may not appear in GetCardList yet;
	// import them from the binding state so the sync still converges. Pan and
	// ExpDate stay empty until a later sync fills them from GetCardList.
	known := make(map[string]bool, len(importable))
	for _, card := range importable {
		known[card.RebillID] = true
	}
	completedRebill := make(map[string]bool, len(completed))
	for _, state := range completed {
		completedRebill[state.RebillID] = true
		if !known[state.RebillID] {
			importable = append(importable, ProviderCard{
				CardID:   state.CardID,
				RebillID: state.RebillID,
				Status:   ProviderCardStatusActive,
			})
		}
	}

	if len(importable) == 0 && len(stale) == 0 {
		return s.ListPaymentMethods(ctx, userID)
	}

	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind payment methods transaction: %w", err)
	}
	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	// The import upserts are deliberately not audited as payment_method.added:
	// the idempotent UpsertByTokenHash (ON CONFLICT) cannot distinguish an
	// insert from an update, and the AddCard webhook flow already records the
	// user-visible addition.
	var freshest *domain.PaymentMethod
	var boundMethod *domain.PaymentMethod
	for _, card := range importable {
		pm, err := domain.NewPaymentMethod(userID, s.provider.Name(), card.RebillID, card.Pan, now)
		if err != nil {
			return nil, fmt.Errorf("create payment method from provider card: %w", err)
		}
		pm.ProviderCardID = card.CardID
		pm.ExpDate = card.ExpDate
		pm, err = txPaymentMethods.UpsertByTokenHash(ctx, pm)
		if err != nil {
			return nil, fmt.Errorf("upsert synced payment method: %w", err)
		}
		if freshest == nil || !pm.CreatedAt.Before(freshest.CreatedAt) {
			freshest = &pm
		}
		if completedRebill[card.RebillID] {
			boundMethod = &pm
		}
	}

	// Drop the binding placeholders: completed ones are replaced by the real
	// card rows upserted above; rejected or expired ones are abandoned and can
	// be restarted via AddPaymentMethod. An already-missing row is fine: a
	// concurrent sync may have dropped it first.
	for _, id := range stale {
		if err := txPaymentMethods.Delete(ctx, userID, id); err != nil && !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("delete resolved card binding placeholder: %w", err)
		}
	}

	methods, err := txPaymentMethods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods after sync: %w", err)
	}

	hasActive := false
	for _, m := range methods {
		if m.IsActive {
			hasActive = true
			break
		}
	}

	// A freshly completed binding becomes the active method, mirroring the
	// AddCard webhook flow. Otherwise only bootstrap activation when nothing
	// is active yet: an explicit user choice (or a webhook-activated method)
	// must not be overridden by a sync.
	activateTarget := boundMethod
	if activateTarget == nil && !hasActive {
		activateTarget = freshest
	}
	if activateTarget != nil {
		if err := txPaymentMethods.SetActive(ctx, userID, activateTarget.ID); err != nil {
			return nil, fmt.Errorf("activate synced payment method: %w", err)
		}

		// Link the activated card to the subscription so renewals charge the
		// right method. A missing subscription is not fatal, same as in the
		// AddCard webhook flow.
		sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("get subscription for synced payment method: %w", err)
			}
			s.deps.log.WarnContext(ctx, "sync payment methods: subscription not found; skipping active method link",
				slog.String("user_id", userID.String()),
				slog.String("payment_method_id", activateTarget.ID.String()))
		} else {
			sub.SetActivePaymentMethod(activateTarget.ID)
			if err := txSubscriptions.Update(ctx, sub); err != nil {
				return nil, fmt.Errorf("update subscription active payment method: %w", err)
			}
		}

		methods, err = txPaymentMethods.ListByUserID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("list payment methods after activation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit sync payment methods transaction: %w", err)
	}

	return filterPendingCardBindings(methods), nil
}
