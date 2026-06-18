package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var (
	ErrNotFound                   = errors.New("not found")
	ErrPaymentMethodInUse         = errors.New("payment method is in use")
	ErrPaymentMethodAlreadyExists = errors.New("payment method already exists")
)

type TariffRepository interface {
	GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error)
	WithTx(tx transaction.Tx) TariffRepository
}

type SubscriptionRepository interface {
	Create(ctx context.Context, sub domain.Subscription) (domain.Subscription, error)
	Update(ctx context.Context, sub domain.Subscription) error
	WithTx(tx transaction.Tx) SubscriptionRepository
}

type PaymentMethodRepository interface {
	Create(ctx context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	SetActive(ctx context.Context, userID, methodID uuid.UUID) error
	DeactivateAllForUser(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, userID, methodID uuid.UUID) error
	WithTx(tx transaction.Tx) PaymentMethodRepository
}

type SubscriptionPaymentRepository interface {
	Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error)
	MarkSucceeded(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errorCode *string) error
	UpdateProviderPaymentID(ctx context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error)
	WithTx(tx transaction.Tx) SubscriptionPaymentRepository
}
