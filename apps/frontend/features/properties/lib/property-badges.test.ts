import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import {
  hasPropertyAttentionDot,
  propertyBadges,
  rentalMonthsLeftLabel,
} from './property-badges';

const TODAY = '2026-09-10';

function makeProperty(overrides: Partial<Property> & { id: string }): Property {
  return {
    name: 'Квартира',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    pinned_at: null,
    ...overrides,
  };
}

describe('rentalMonthsLeftLabel', () => {
  it('полные месяцы до планового окончания', () => {
    expect(rentalMonthsLeftLabel('2026-09-10', '2027-09-10')).toBe('Осталось 12 месяцев аренды');
  });

  it('склоняет месяцы: 1 месяц, 2 месяца, 5 месяцев', () => {
    expect(rentalMonthsLeftLabel('2026-09-10', '2026-10-10')).toBe('Остался 1 месяц аренды');
    expect(rentalMonthsLeftLabel('2026-09-10', '2026-11-10')).toBe('Осталось 2 месяца аренды');
    expect(rentalMonthsLeftLabel('2026-09-10', '2027-02-10')).toBe('Осталось 5 месяцев аренды');
  });

  it('N=0 — «Осталось меньше месяца» (резолюция #584)', () => {
    expect(rentalMonthsLeftLabel('2026-09-10', '2026-09-25')).toBe('Осталось меньше месяца');
  });
});

describe('propertyBadges', () => {
  it('needs_attention — залитый синий «Аренда завершена»', () => {
    const property = makeProperty({
      id: '1',
      occupancy: { status: 'needs_attention', start_date: '2025-09-01', planned_end_date: '2026-09-01' },
    });
    expect(propertyBadges(property, TODAY)).toEqual([
      { key: 'rental-completed', label: 'Аренда завершена', tone: 'accent-filled' },
    ]);
  });

  it('активная с плановым окончанием — «Осталось N месяцев аренды»', () => {
    const property = makeProperty({
      id: '2',
      occupancy: { status: 'active', start_date: '2025-09-01', planned_end_date: '2027-09-10' },
    });
    expect(propertyBadges(property, TODAY)).toEqual([
      { key: 'rental-months', label: 'Осталось 12 месяцев аренды', tone: 'accent' },
    ]);
  });

  it('активная бессрочная — без арендного бейджа', () => {
    const property = makeProperty({
      id: '3',
      occupancy: { status: 'active', start_date: '2025-09-01', planned_end_date: null },
    });
    expect(propertyBadges(property, TODAY)).toEqual([]);
  });

  it('upcoming — «Аренда с DD.MM» короткой датой', () => {
    const property = makeProperty({
      id: '4',
      occupancy: { status: 'upcoming', start_date: '2026-10-05', planned_end_date: null },
    });
    expect(propertyBadges(property, TODAY)).toEqual([
      { key: 'rental-upcoming', label: 'Аренда с 05.10', tone: 'accent' },
    ]);
  });

  it('явно завершённая аренда (none) — без бейджа', () => {
    const property = makeProperty({
      id: '5',
      occupancy: { status: 'none', start_date: null, planned_end_date: null },
    });
    expect(propertyBadges(property, TODAY)).toEqual([]);
  });

  it('на ремонте — жёлтый бейдж, соседствует с арендным', () => {
    const property = makeProperty({
      id: '6',
      status: 'maintenance',
      occupancy: { status: 'active', start_date: '2025-09-01', planned_end_date: '2027-09-10' },
    });
    expect(propertyBadges(property, TODAY)).toEqual([
      { key: 'rental-months', label: 'Осталось 12 месяцев аренды', tone: 'accent' },
      { key: 'maintenance', label: 'Объект на ремонте', tone: 'warning' },
    ]);
  });

  it('без обогащения занятости — только жизненный цикл', () => {
    expect(propertyBadges(makeProperty({ id: '7' }), TODAY)).toEqual([]);
  });

  it('без today «Осталось N месяцев» откладывается, остальные бейджи на месте', () => {
    const months = makeProperty({
      id: '13',
      occupancy: { status: 'active', start_date: '2025-09-01', planned_end_date: '2027-09-10' },
    });
    expect(propertyBadges(months)).toEqual([]);

    const completed = makeProperty({
      id: '14',
      occupancy: { status: 'needs_attention', start_date: null, planned_end_date: null },
    });
    expect(propertyBadges(completed)).toEqual([
      { key: 'rental-completed', label: 'Аренда завершена', tone: 'accent-filled' },
    ]);
  });
});

describe('hasPropertyAttentionDot (резолюция #584)', () => {
  it('точка от просрочки операций', () => {
    expect(hasPropertyAttentionDot(makeProperty({ id: '8', has_overdue_operations: true }))).toBe(true);
  });

  it('точка от «подошла к концу»', () => {
    const property = makeProperty({
      id: '9',
      occupancy: { status: 'needs_attention', start_date: null, planned_end_date: null },
    });
    expect(hasPropertyAttentionDot(property)).toBe(true);
  });

  it('активная/будущая/завершённая аренда, ремонт и отсутствие данных точку не ставят', () => {
    expect(hasPropertyAttentionDot(makeProperty({ id: '10' }))).toBe(false);
    expect(
      hasPropertyAttentionDot(
        makeProperty({ id: '11', status: 'maintenance', occupancy: { status: 'none', start_date: null, planned_end_date: null } }),
      ),
    ).toBe(false);
    expect(
      hasPropertyAttentionDot(
        makeProperty({ id: '12', occupancy: { status: 'active', start_date: '2025-01-01', planned_end_date: null } }),
      ),
    ).toBe(false);
  });
});
