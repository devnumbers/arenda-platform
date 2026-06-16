package postgres

import (
	"context"
	"errors"
	"math/big"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TariffRepository persists tariffs.
type TariffRepository struct {
	db postgres.DBTX
}

// NewTariffRepository creates a new tariff repository.
func NewTariffRepository(db postgres.DBTX) *TariffRepository {
	return &TariffRepository{db: db}
}

func (r *TariffRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *TariffRepository) WithTx(tx transaction.Tx) application.TariffRepository {
	return NewTariffRepository(tx.(postgres.DBTX))
}

func (r *TariffRepository) GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error) {
	row, err := r.q().GetTariffByName(ctx, string(name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Tariff{}, application.ErrNotFound
		}
		return domain.Tariff{}, err
	}
	monthly, err := numericToKopecks(row.MonthlyPrice)
	if err != nil {
		return domain.Tariff{}, err
	}
	yearly, err := numericToKopecks(row.YearlyPrice)
	if err != nil {
		return domain.Tariff{}, err
	}
	return domain.Tariff{
		ID:                  uuid.UUID(row.ID.Bytes),
		Name:                domain.TariffName(row.Name),
		ActivePropertyLimit: int(row.ActivePropertyLimit),
		MonthlyPriceKopecks: monthly,
		YearlyPriceKopecks:  yearly,
	}, nil
}

// SubscriptionRepository persists subscriptions.
type SubscriptionRepository struct {
	db postgres.DBTX
}

// NewSubscriptionRepository creates a new subscription repository.
func NewSubscriptionRepository(db postgres.DBTX) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SubscriptionRepository) WithTx(tx transaction.Tx) application.SubscriptionRepository {
	return NewSubscriptionRepository(tx.(postgres.DBTX))
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub domain.Subscription) error {
	_, err := r.q().CreateSubscription(ctx, postgres.CreateSubscriptionParams{
		UserID:   pgtype.UUID{Bytes: sub.UserID, Valid: true},
		TariffID: pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:   string(sub.Source),
		Status:   string(sub.Status),
	})
	return err
}

func numericToKopecks(n pgtype.Numeric) (int64, error) {
	if !n.Valid {
		return 0, errors.New("invalid numeric")
	}
	if n.NaN {
		return 0, errors.New("numeric is NaN")
	}
	if n.InfinityModifier != pgtype.Finite {
		return 0, errors.New("numeric is infinite")
	}
	if n.Int == nil {
		return 0, errors.New("numeric has nil coefficient")
	}

	// Numeric value is Int * 10^Exp. Kopecks are value * 100,
	// so compute Int * 10^(Exp + 2) using exact integer arithmetic.
	result := new(big.Int).Set(n.Int)
	exp := int64(n.Exp) + 2

	if exp >= 0 {
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(exp), nil)
		result.Mul(result, factor)
	} else {
		divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(-exp), nil)
		result.Div(result, divisor)
	}

	if !result.IsInt64() {
		return 0, errors.New("numeric value overflows int64")
	}
	return result.Int64(), nil
}
