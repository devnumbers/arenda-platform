package postgres

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

func TestMapTariff(t *testing.T) {
	id := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")

	row := genpostgres.Tariff{
		ID:                  pgtype.UUID{Bytes: id, Valid: true},
		Name:                "pro",
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 49000,
		YearlyPriceKopecks:  440000,
	}

	want := domain.Tariff{
		ID:                  id,
		Name:                domain.TariffName("pro"),
		ActivePropertyLimit: 5,
		MonthlyPriceKopecks: 49000,
		YearlyPriceKopecks:  440000,
	}

	got := mapTariff(row)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapTariff() = %+v, want %+v", got, want)
	}
}

func TestMapTariffs(t *testing.T) {
	proID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	basicID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a12")

	rows := []genpostgres.Tariff{
		{
			ID:                  pgtype.UUID{Bytes: basicID, Valid: true},
			Name:                "basic",
			ActivePropertyLimit: 1,
			MonthlyPriceKopecks: 0,
			YearlyPriceKopecks:  0,
		},
		{
			ID:                  pgtype.UUID{Bytes: proID, Valid: true},
			Name:                "pro",
			ActivePropertyLimit: 5,
			MonthlyPriceKopecks: 49000,
			YearlyPriceKopecks:  440000,
		},
	}

	want := []domain.Tariff{
		{
			ID:                  basicID,
			Name:                domain.TariffName("basic"),
			ActivePropertyLimit: 1,
			MonthlyPriceKopecks: 0,
			YearlyPriceKopecks:  0,
		},
		{
			ID:                  proID,
			Name:                domain.TariffName("pro"),
			ActivePropertyLimit: 5,
			MonthlyPriceKopecks: 49000,
			YearlyPriceKopecks:  440000,
		},
	}

	got := mapTariffs(rows)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapTariffs() = %+v, want %+v", got, want)
	}
}
