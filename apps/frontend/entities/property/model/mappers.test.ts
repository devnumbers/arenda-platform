import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import {
  mapPropertyOperationsSummaryResponse,
  mapPropertyResponse,
} from './mappers';

function makeDto(
  overrides: Partial<components['schemas']['PropertyResponse']> = {},
): components['schemas']['PropertyResponse'] {
  return {
    id: 'property-1',
    name: 'Квартира на Ленина',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    occupancy: 'free',
    active_lease: null,
    overdue_rent_count: 0,
    members_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('mapPropertyResponse access', () => {
  it('maps access role and owner_name to camelCase ownerName', () => {
    const property = mapPropertyResponse(
      makeDto({ access: { role: 'viewer', owner_name: 'Иван Петров' } }),
    );

    expect(property.access).toEqual({ role: 'viewer', ownerName: 'Иван Петров' });
  });

  it('maps access without owner_name', () => {
    const property = mapPropertyResponse(makeDto({ access: { role: 'owner' } }));

    expect(property.access).toEqual({ role: 'owner', ownerName: undefined });
  });

  it('leaves access undefined when the DTO has no access', () => {
    const property = mapPropertyResponse(makeDto());

    expect(property.access).toBeUndefined();
  });
});

describe('mapPropertyOperationsSummaryResponse', () => {
  it('maps snake_case summary DTO to camelCase entity', () => {
    const summary = mapPropertyOperationsSummaryResponse({
      monthly_profit_kopecks: 120000,
      all_time_profit_kopecks: 1440000,
      all_time_income_kopecks: 2440000,
      all_time_expense_kopecks: 1000000,
      overdue_rent_count: 1,
      overdue_total_count: 2,
      next_payment_date: '2026-09-10',
    });

    expect(summary).toEqual({
      monthlyProfitKopecks: 120000,
      allTimeProfitKopecks: 1440000,
      allTimeIncomeKopecks: 2440000,
      allTimeExpenseKopecks: 1000000,
      overdueRentCount: 1,
      overdueTotalCount: 2,
      nextPaymentDate: '2026-09-10',
    });
  });

  it('maps a summary without next payment date', () => {
    const summary = mapPropertyOperationsSummaryResponse({
      monthly_profit_kopecks: 0,
      all_time_profit_kopecks: 0,
      all_time_income_kopecks: 0,
      all_time_expense_kopecks: 0,
      overdue_rent_count: 0,
      overdue_total_count: 0,
    });

    expect(summary.nextPaymentDate).toBeNull();
  });
});
