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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// methodBindingProvider is the narrow provider slice the payment-method
// service needs: starting and polling binding sessions, detaching saved
// methods, and the provider identity. Declared here, at the consumer, per
// ADR 0035.
type methodBindingProvider interface {
	PaymentMethodBinder
	PaymentMethodBindingReader
	PaymentMethodRemover
	ProviderNamer
}

// RawTokenMethodAcceptor is the provider capability of accepting a raw charge
// token synchronously (the fake adapter for local runs). Bank-form providers
// such as T-Kassa do not implement it: per the frozen contract a providerToken
// they receive is ignored and the binding flow runs instead — an arbitrary
// client-supplied token must never become a chargeable method there.
type RawTokenMethodAcceptor interface {
	AddPaymentMethodFromToken(ctx context.Context, customerRef, token string) (SavedMethod, error)
}

// PaymentMethodService manages the user's saved payment methods (issue #251):
// the card-binding flow (initiation, polling, completion), activation of the
// single active method, deletion, and the method list.
type PaymentMethodService struct {
	txStoreFactory
	provider methodBindingProvider
	clock    clock.Clock
	config   Config
	log      *slog.Logger
}

// PaymentMethodServiceConfig carries the non-transactional dependencies of the
// payment-method service.
type PaymentMethodServiceConfig struct {
	Config Config
	Clock  clock.Clock
	Log    *slog.Logger
}

// NewPaymentMethodService creates a payment-method service over the shared
// factory. A zero Config substitutes DefaultConfig, like the sibling billing
// services — a zero CardBindingSessionLimit would otherwise refuse every
// binding.
func NewPaymentMethodService(factory txStoreFactory, provider methodBindingProvider, cfg PaymentMethodServiceConfig) *PaymentMethodService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.Config == (Config{}) {
		cfg.Config = DefaultConfig()
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &PaymentMethodService{
		txStoreFactory: factory,
		provider:       provider,
		clock:          cfg.Clock,
		config:         cfg.Config,
		log:            cfg.Log,
	}
}

// AddPaymentMethod serves POST /subscription/payment-methods (issue #251).
// A raw provider token creates the method directly — but only through a
// provider that accepts raw tokens (the fake); bank-form providers ignore the
// token per the contract and run the binding session, whose confirmation URL
// the payer follows. Both paths converge on the same end state — a saved
// method that is the user's single active one and the subscription's charge
// target.
func (s *PaymentMethodService) AddPaymentMethod(
	ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest,
) (AddPaymentMethodResult, error) {
	if s.provider == nil {
		// No provider wired (pre-#250 construction): refuse before anything is
		// persisted, mirroring the tariff-change flow.
		return AddPaymentMethodResult{}, ErrPaymentUnavailable
	}
	if acceptor, ok := s.provider.(RawTokenMethodAcceptor); ok && req.ProviderToken != "" {
		return s.addFromToken(ctx, acceptor, userID, req.ProviderToken)
	}
	return s.startBinding(ctx, userID)
}

// addFromToken creates the payment method from a raw provider token — the
// synchronous counterpart of a completed binding, so local runs and tests
// reach the same end state. The token itself is accepted by the provider (the
// capability the caller checked), never trusted raw.
func (s *PaymentMethodService) addFromToken(
	ctx context.Context, acceptor RawTokenMethodAcceptor, userID uuid.UUID, token string,
) (AddPaymentMethodResult, error) {
	saved, err := acceptor.AddPaymentMethodFromToken(ctx, userID.String(), token)
	if err != nil {
		return AddPaymentMethodResult{}, fmt.Errorf("add payment method from token at provider: %w", err)
	}
	now := s.clock.Now().UTC()
	method, err := domain.NewPaymentMethod(userID, s.provider.Name(), saved.ChargeToken, now)
	if err != nil {
		return AddPaymentMethodResult{}, err
	}
	method.ProviderCardID = saved.ProviderMethodID
	method.DisplayMask = saved.MaskedPan
	method.ExpDate = saved.ExpDate

	var created domain.PaymentMethod
	err = s.runInTx(ctx, func(stores *txStores) error {
		var applyErr error
		// No binding session backs the token path; nil keeps the completion
		// shared with the binding flows.
		created, applyErr = applyCompletedCardBinding(ctx, stores, s.log, method, nil, now)
		return applyErr
	})
	if err != nil {
		return AddPaymentMethodResult{}, err
	}
	return AddPaymentMethodResult{PaymentMethod: &created}, nil
}

// startBinding initiates the card-binding session at the provider and persists
// it with its TTL (issue #251). The provider call runs before the row is
// saved: a crash in between leaves a provider-side session that expires
// harmlessly — never a local session without a provider counterpart. The
// per-user sliding window (ticket #427) is checked before the provider call,
// so an over-limit request never consumes provider quota. The check is
// deliberately soft — count and insert share no lock, so parallel requests
// may slightly overshoot, which an abuse limit tolerates.
func (s *PaymentMethodService) startBinding(ctx context.Context, userID uuid.UUID) (AddPaymentMethodResult, error) {
	now := s.clock.Now().UTC()
	started, err := s.bindings.CountStartedSince(ctx, userID, now.Add(-s.config.CardBindingSessionWindow))
	if err != nil {
		return AddPaymentMethodResult{}, fmt.Errorf("check card binding session limit: %w", err)
	}
	if started >= s.config.CardBindingSessionLimit {
		return AddPaymentMethodResult{}, ErrBindingSessionLimitExceeded
	}

	result, err := s.provider.BindPaymentMethod(ctx, BindMethodRequest{CustomerRef: userID.String()})
	if err != nil {
		return AddPaymentMethodResult{}, fmt.Errorf("bind payment method at provider: %w", err)
	}

	session, err := domain.NewCardBindingSession(userID, s.provider.Name(), result.BindingID, now.Add(s.config.CardBindingTTL), now)
	if err != nil {
		return AddPaymentMethodResult{}, err
	}
	if _, err := s.bindings.Create(ctx, session); err != nil {
		return AddPaymentMethodResult{}, fmt.Errorf("save card binding session: %w", err)
	}
	return AddPaymentMethodResult{ConfirmURL: result.FormURL}, nil
}

// ListPaymentMethods serves GET /subscription/payment-methods: the user's
// saved methods, newest first.
func (s *PaymentMethodService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	methods, err := s.methods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods: %w", err)
	}
	return methods, nil
}

// ActivatePaymentMethod serves POST /subscription/payment-methods/{id}/activate
// (issue #251): the method becomes the user's single active one — the partial
// unique index is the durable exactly-one-active invariant — and the
// subscription's charge target, so renewals run on the newly chosen card.
func (s *PaymentMethodService) ActivatePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		if _, err := stores.methodForUpdate(ctx, userID, methodID); err != nil {
			return err
		}
		if err := stores.methods.SetActive(ctx, userID, methodID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrPaymentMethodNotFound
			}
			return fmt.Errorf("set active payment method: %w", err)
		}
		// Unlike the binding completion, an explicit activation requires the
		// subscription: renewals are what the user is choosing the method for.
		sub, err := stores.subscriptionForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		sub.SetActivePaymentMethod(methodID)
		if err := stores.subscriptions.Update(ctx, sub); err != nil {
			return fmt.Errorf("update subscription active payment method: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPaymentMethodActivated,
			EntityType: auditdomain.EntityPaymentMethod,
			EntityID:   &methodID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// DeletePaymentMethod serves DELETE /subscription/payment-methods/{id}
// (issue #251). The active method cannot be deleted until another one is
// activated — subscription renewals need a charge target. The provider-side
// card is detached best-effort after the local row is gone, so a provider
// failure cannot resurrect the deleted method.
func (s *PaymentMethodService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	var deleted domain.PaymentMethod
	err := s.runInTx(ctx, func(stores *txStores) error {
		method, err := stores.methodForUpdate(ctx, userID, methodID)
		if err != nil {
			return err
		}
		if method.IsActive {
			return ErrPaymentMethodInUse
		}
		if err := stores.methods.Delete(ctx, userID, methodID); err != nil {
			return fmt.Errorf("delete payment method: %w", err)
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleOwner,
			Action:     auditdomain.ActionPaymentMethodDeleted,
			EntityType: auditdomain.EntityPaymentMethod,
			EntityID:   &methodID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		deleted = method
		return nil
	})
	if err != nil {
		return err
	}

	// The local row is gone; detach the card at the provider best-effort. A
	// provider failure must not fail the operation since the method is
	// already deleted.
	if s.provider != nil && deleted.ProviderCardID != "" {
		if err := s.provider.RemovePaymentMethod(ctx, userID.String(), deleted.ProviderCardID); err != nil {
			if errors.Is(err, ErrProviderMethodNotFound) {
				s.log.WarnContext(ctx, "provider card already removed; continuing local deletion",
					slog.String("payment_method_id", methodID.String()))
			} else {
				s.log.ErrorContext(ctx, "failed to remove provider card after payment method deletion",
					slog.String("payment_method_id", methodID.String()),
					slog.String("error", err.Error()))
			}
		}
	}
	return nil
}

// SyncPaymentMethods serves POST /subscription/payment-methods/sync
// (issue #251): the self-healing path when the add-card webhook was not
// delivered. Every open binding session of the user is polled at the
// provider: a completed binding produces (and activates) the payment method
// exactly like the webhook would, a failed or provider-forgotten binding is
// closed, and an expired session is closed without polling — it can never
// complete. The method is idempotent and returns the user's up-to-date list.
func (s *PaymentMethodService) SyncPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	sessions, err := s.bindings.ListOpenByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list card binding sessions: %w", err)
	}

	now := s.clock.Now().UTC()
	for _, session := range sessions {
		if s.provider == nil {
			break // Nothing to poll without a provider; sessions expire by TTL.
		}
		if session.IsExpired(now) {
			// The binding form has expired: the binding can never complete,
			// so close the session without polling the provider.
			s.closeBindingSession(ctx, session, now)
			continue
		}
		state, pollErr := s.provider.PaymentMethodBinding(ctx, session.RequestKey)
		if pollErr != nil {
			if errors.Is(pollErr, ErrProviderBindingNotFound) {
				// The provider no longer knows the request key: the binding
				// expired provider-side and can never complete.
				s.closeBindingSession(ctx, session, now)
				continue
			}
			s.log.WarnContext(ctx, "sync payment methods: failed to poll binding state; keeping session open",
				slog.String("user_id", userID.String()),
				slog.String("binding_id", session.RequestKey),
				slog.String("error", pollErr.Error()))
			continue
		}
		switch state.Status {
		case MethodBindingCompleted:
			if state.Method == nil || state.Method.ChargeToken == "" {
				s.log.WarnContext(ctx, "sync payment methods: completed binding has no charge token; keeping session open",
					slog.String("user_id", userID.String()),
					slog.String("binding_id", session.RequestKey))
				continue
			}
			s.completeBindingSession(ctx, session, *state.Method, now)
		case MethodBindingFailed:
			s.closeBindingSession(ctx, session, now)
		case MethodBindingPending:
			// Still in flight; the session stays open until the webhook, a
			// later sync, or its TTL resolves it.
		}
	}

	methods, err := s.methods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods after sync: %w", err)
	}
	return methods, nil
}

// closeBindingSession closes an open session as rejected — the terminal step
// for bindings that expired, failed, or were forgotten by the provider.
// Failures are logged, not propagated: the sync continues with the remaining
// sessions.
func (s *PaymentMethodService) closeBindingSession(ctx context.Context, session domain.CardBindingSession, now time.Time) {
	if err := s.runInTx(ctx, func(stores *txStores) error {
		current, err := stores.bindings.GetByRequestKeyForUpdate(ctx, session.Provider, session.RequestKey)
		if err != nil {
			return err
		}
		if err := current.MarkRejected(now); err != nil {
			return err
		}
		if err := stores.bindings.UpdateStatus(ctx, current); err != nil {
			return fmt.Errorf("close card binding session: %w", err)
		}
		return nil
	}); err != nil {
		s.log.WarnContext(ctx, "sync payment methods: failed to close binding session",
			slog.String("binding_id", session.RequestKey),
			slog.String("error", err.Error()))
	}
}

// completeBindingSession applies a provider-confirmed binding through the
// shared completion path. Failures are logged, not propagated: the sync
// continues with the remaining sessions.
func (s *PaymentMethodService) completeBindingSession(
	ctx context.Context, session domain.CardBindingSession, method SavedMethod, now time.Time,
) {
	err := s.runInTx(ctx, func(stores *txStores) error {
		current, err := stores.bindings.GetByRequestKeyForUpdate(ctx, session.Provider, session.RequestKey)
		if err != nil {
			return err
		}
		bound, err := domain.NewPaymentMethod(current.UserID, current.Provider, method.ChargeToken, now)
		if err != nil {
			return err
		}
		bound.ProviderCardID = method.ProviderMethodID
		bound.DisplayMask = method.MaskedPan
		bound.ExpDate = method.ExpDate
		_, err = applyCompletedCardBinding(ctx, stores, s.log, bound, &current, now)
		return err
	})
	if err != nil {
		s.log.WarnContext(ctx, "sync payment methods: failed to complete binding session",
			slog.String("binding_id", session.RequestKey),
			slog.String("error", err.Error()))
	}
}

// applyCompletedCardBinding applies a confirmed card binding inside the
// caller's transaction (issue #251): the payment method is upserted by token
// hash (a re-bound card converges on its row instead of duplicating), the
// session is marked completed, the method becomes the user's single active
// one, and the subscription's charge target points at it so renewals charge
// the new card. Session is nil on the synchronous token path. A session that
// is already resolved (a concurrent delivery won) or expired writes nothing —
// a repeated delivery is a no-op and an expired session never produces a
// payment method. A missing subscription is not fatal for the binding itself:
// the method is still saved and active, only the renewal link is skipped (the
// webhook flow must not make the provider retry forever).
func applyCompletedCardBinding(
	ctx context.Context, stores *txStores, log *slog.Logger,
	method domain.PaymentMethod, session *domain.CardBindingSession, now time.Time,
) (domain.PaymentMethod, error) {
	if session != nil && !session.CanComplete(now) {
		// The session is already resolved (a concurrent delivery won) or its
		// lifetime is over — an expired session must never produce a payment
		// method, so nothing is written at all.
		return domain.PaymentMethod{}, nil
	}

	saved, err := stores.methods.UpsertByTokenHash(ctx, method)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("save payment method: %w", err)
	}

	if session != nil {
		if err := session.MarkCompleted(now); err != nil {
			return domain.PaymentMethod{}, err
		}
		if err := stores.bindings.UpdateStatus(ctx, *session); err != nil {
			return domain.PaymentMethod{}, fmt.Errorf("complete card binding session: %w", err)
		}
	}

	if err := stores.methods.SetActive(ctx, saved.UserID, saved.ID); err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("activate payment method: %w", err)
	}
	saved.IsActive = true

	if err := linkSubscriptionToMethod(ctx, stores, log, saved.UserID, saved.ID); err != nil {
		return domain.PaymentMethod{}, err
	}

	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorRole:  auditdomain.ActorRoleSystem,
		Action:     auditdomain.ActionPaymentMethodAdded,
		EntityType: auditdomain.EntityPaymentMethod,
		EntityID:   &saved.ID,
		Context:    map[string]any{"payment_method_id": saved.ID, auditKeyProvider: string(saved.Provider)},
	}); err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("record audit: %w", err)
	}
	return saved, nil
}

// linkSubscriptionToMethod points the user's subscription at the activated
// method so renewals charge it. A user without a subscription row is skipped
// with a warning: the binding completion must not fail over it.
func linkSubscriptionToMethod(ctx context.Context, stores *txStores, log *slog.Logger, userID, methodID uuid.UUID) error {
	sub, err := stores.subscriptions.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			log.WarnContext(ctx, "subscription not found; skipping active payment method link",
				slog.String("user_id", userID.String()),
				slog.String("payment_method_id", methodID.String()))
			return nil
		}
		return fmt.Errorf("get subscription for payment method: %w", err)
	}
	sub.SetActivePaymentMethod(methodID)
	if err := stores.subscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription active payment method: %w", err)
	}
	return nil
}
