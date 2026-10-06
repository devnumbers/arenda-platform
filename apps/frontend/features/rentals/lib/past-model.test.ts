import { describe, expect, it } from 'vitest';
import { makeRental, type Rental } from '@/entities/rental';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  completedRentalMonths,
  completedRentalsOf,
  pastRentalCardTitle,
  pastRentalRows,
  pastRentalTitle,
} from './past-model';

/** Фикстура завершённой аренды из макета #535 (1302:52462): 56 000 ₽,
 * 10.05.2026 → 10.05.2028 (24 месяца), завершена 10.05.2028, арендатор.
 * Ревизия #1161: платёж удалён Завершением — paymentId null, сумма и день
 * оплаты читаются из Архива условий. Остальное — канон makeRental. */
function completedFixture(overrides: Partial<Rental> = {}): Rental {
  return makeRental({
    id: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a55',
    propertyId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a01',
    status: 'completed',
    startDate: '2026-05-10',
    plannedEndDate: '2028-05-10',
    completedDate: '2028-05-10',
    utilities: 'meters_only',
    depositKopecks: 0,
    commissionKopecks: 0,
    depositReturnKopecks: 5_600_000,
    tenant: {
      contactId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a66',
      firstName: 'Александр',
      lastName: '',
      phone: '+79934302010',
    },
    comment: '',
    rentPayment: {
      paymentId: null,
      amountKopecks: 5_600_000,
      paymentDay: 10,
      autoPay: false,
      nextPayment: null,
    },
    progress: { paidMonths: 24, totalMonths: 24, monthsRemaining: 0, overdueMonths: null },
    today: '2029-01-15',
    createdAt: '2026-05-10T10:00:00Z',
    ...overrides,
  });
}

describe('completedRentalsOf', () => {
  it('оставляет только завершённые, порядок сервера (свежие сверху) сохраняется', () => {
    const active = completedFixture({ id: 'a', status: 'active' });
    const fresh = completedFixture({ id: 'b', completedDate: '2028-05-10' });
    const old = completedFixture({ id: 'c', completedDate: '2026-08-01' });
    const items = [active, fresh, old];
    expect(completedRentalsOf(items)).toEqual([fresh, old]);
  });

  it('на пустом списке — пусто', () => {
    expect(completedRentalsOf([])).toEqual([]);
  });
});

describe('completedRentalMonths', () => {
  it('полные месяцы между началом и датой завершения', () => {
    expect(completedRentalMonths(completedFixture())).toBe(24);
  });

  it('считает фактическую дату завершения, не плановое окончание', () => {
    const rental = completedFixture({ completedDate: '2028-01-10' });
    expect(completedRentalMonths(rental)).toBe(20);
  });

  it('без даты завершения — плановое окончание', () => {
    const rental = completedFixture({ completedDate: null });
    expect(completedRentalMonths(rental)).toBe(24);
  });

  it('меньше полного месяца — ноль', () => {
    const rental = completedFixture({
      startDate: '2028-05-10',
      completedDate: '2028-05-25',
      plannedEndDate: null,
    });
    expect(completedRentalMonths(rental)).toBe(0);
  });
});

describe('pastRentalTitle', () => {
  it('карточка списка — срок всегда, без платежей тоже (тикет: список о сроках)', () => {
    expect(pastRentalCardTitle(completedFixture())).toBe('24 месяца');
    expect(
      pastRentalCardTitle(
        completedFixture({ startDate: '2028-05-10', completedDate: '2028-05-25', plannedEndDate: null }),
      ),
    ).toBe('Меньше месяца');
  });

  it('без оплаченных операций — «Не было платежей» (1550:94517)', () => {
    expect(pastRentalTitle(completedFixture(), 0)).toBe('Не было платежей');
  });

  it('с платежами — срок в месяцах (1302:52462)', () => {
    expect(pastRentalTitle(completedFixture(), 24)).toBe('24 месяца');
    expect(pastRentalTitle(completedFixture({ completedDate: '2027-04-10' }), 3)).toBe(
      '11 месяцев',
    );
  });

  it('согласование числительного: 21 месяц, 1 месяц', () => {
    const rental = completedFixture({ completedDate: '2028-02-10' });
    expect(pastRentalTitle(rental, 3)).toBe('21 месяц');
    const one = completedFixture({
      startDate: '2028-04-10',
      completedDate: '2028-05-10',
      plannedEndDate: null,
    });
    expect(pastRentalTitle(one, 1)).toBe('1 месяц');
  });

  it('неполный месяц с платежами — «Меньше месяца»', () => {
    const rental = completedFixture({
      startDate: '2028-05-10',
      completedDate: '2028-05-25',
      plannedEndDate: null,
    });
    expect(pastRentalTitle(rental, 1)).toBe('Меньше месяца');
  });
});

describe('pastRentalRows', () => {
  it('три строки карточки списка: плата, начало, окончание (1302:52462)', () => {
    expect(pastRentalRows(completedFixture())).toEqual([
      { label: 'Арендная плата', value: `${formatMoneyKopecks(5_600_000)} в месяц` },
      { label: 'Начало аренды', value: '10 мая, 2026' },
      { label: 'Окончание аренды', value: '10 мая, 2028' },
    ]);
  });

  it('окончание на карточке — фактическая дата завершения, не плановая (F2 аудита #801)', () => {
    const rental = completedFixture({ completedDate: '2026-11-23' });
    const rows = pastRentalRows(rental);
    expect(rows[2]).toEqual({ label: 'Окончание аренды', value: '23 ноября, 2026' });
  });

  it('бессрочная завершённая — фактическая дата завершения', () => {
    const rental = completedFixture({ plannedEndDate: null });
    const rows = pastRentalRows(rental);
    expect(rows[2]).toEqual({ label: 'Окончание аренды', value: '10 мая, 2028' });
  });

  it('без факта завершения (тип nullable, живьём не бывает) — плановая, затем «Не указано»', () => {
    const planned = pastRentalRows(completedFixture({ completedDate: null }));
    expect(planned[2]).toEqual({ label: 'Окончание аренды', value: '10 мая, 2028' });
    const none = pastRentalRows(
      completedFixture({ completedDate: null, plannedEndDate: null }),
    );
    expect(none[2]).toEqual({ label: 'Окончание аренды', value: 'Не указано' });
  });
});
