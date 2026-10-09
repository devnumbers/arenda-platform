import { describe, expect, it } from 'vitest';
import { makeRental, type Rental } from '@/entities/rental';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import {
  hasProgressCard,
  rentalElapsedLine,
  rentalNextPaymentLine,
  rentalOverdueLine,
  rentalPaidTitle,
  rentalProgressPercent,
  rentalRemainingLine,
  rentalTeaserRows,
  rentalTermsRows,
  rentalTenantTitle,
  rentAmountPerMonth,
} from './rental-view';

/** Фикстура аренды из макета #531 (1232:61291): 56 000 ₽, 10-е число,
 * 10.10 → 10.10+24 мес., оплачено 6 из 24. Остальное — канон makeRental. */
function rentalFixture(overrides: Partial<Rental> = {}): Rental {
  return makeRental({
    id: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a55',
    propertyId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a01',
    startDate: '2026-10-10',
    plannedEndDate: '2028-10-10',
    utilities: 'meters_only',
    depositKopecks: 5_600_000,
    commissionKopecks: 0,
    tenant: {
      contactId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a66',
      firstName: 'Александр',
      lastName: 'Петров',
      phone: '+79934302010',
    },
    comment: 'Дом — панельный',
    rentPayment: {
      paymentId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a88',
      amountKopecks: 5_600_000,
      paymentDay: 10,
      autoPay: false,
      reminderOffsetDays: null,
      nextPayment: {
        operationId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a77',
        date: '2027-02-10',
        amountKopecks: 5_600_000,
        daysUntil: 150,
      },
    },
    today: '2026-09-05',
    createdAt: '2026-09-05T10:00:00Z',
    ...overrides,
  });
}

describe('rentalPaidTitle', () => {
  it('срочная аренда — «N из M месяцев» по числительному итога', () => {
    expect(rentalPaidTitle({ paidMonths: 6, totalMonths: 24, monthsRemaining: 23, overdueMonths: null })).toBe(
      '6 из 24 месяцев',
    );
    expect(rentalPaidTitle({ paidMonths: 6, totalMonths: 1, monthsRemaining: 0, overdueMonths: null })).toBe(
      '6 из 1 месяца',
    );
    expect(rentalPaidTitle({ paidMonths: 1, totalMonths: 2, monthsRemaining: 1, overdueMonths: null })).toBe(
      '1 из 2 месяцев',
    );
    expect(rentalPaidTitle({ paidMonths: 0, totalMonths: 21, monthsRemaining: 21, overdueMonths: null })).toBe(
      '0 из 21 месяца',
    );
    expect(rentalPaidTitle({ paidMonths: 9, totalMonths: 11, monthsRemaining: 2, overdueMonths: null })).toBe(
      '9 из 11 месяцев',
    );
  });

  it('бессрочная аренда — только оплаченные месяцы', () => {
    expect(rentalPaidTitle({ paidMonths: 6, totalMonths: null, monthsRemaining: null, overdueMonths: null })).toBe(
      '6 месяцев',
    );
    expect(rentalPaidTitle({ paidMonths: 1, totalMonths: null, monthsRemaining: null, overdueMonths: null })).toBe(
      '1 месяц',
    );
    expect(rentalPaidTitle({ paidMonths: 0, totalMonths: null, monthsRemaining: null, overdueMonths: null })).toBe(
      '0 месяцев',
    );
  });
});

describe('rentalProgressPercent', () => {
  it('доля оплаченных месяцев срочной аренды, 0…100', () => {
    expect(rentalProgressPercent({ paidMonths: 6, totalMonths: 24, monthsRemaining: 23, overdueMonths: null })).toBe(25);
    expect(rentalProgressPercent({ paidMonths: 0, totalMonths: 12, monthsRemaining: 12, overdueMonths: null })).toBe(0);
    expect(rentalProgressPercent({ paidMonths: 12, totalMonths: 12, monthsRemaining: 0, overdueMonths: null })).toBe(100);
  });

  it('бессрочная аренда — процента нет', () => {
    expect(rentalProgressPercent({ paidMonths: 6, totalMonths: null, monthsRemaining: null, overdueMonths: null })).toBe(
      null,
    );
  });
});

describe('rentalNextPaymentLine', () => {
  it('дни до платежа со склонением', () => {
    expect(
      rentalNextPaymentLine({
        operationId: 'op',
        date: '2027-02-10',
        amountKopecks: 5_600_000,
        daysUntil: 150,
      }),
    ).toBe('150 дней до следующего платежа');
    expect(
      rentalNextPaymentLine({
        operationId: 'op',
        date: '2026-09-07',
        amountKopecks: 1,
        daysUntil: 2,
      }),
    ).toBe('2 дня до следующего платежа');
    expect(
      rentalNextPaymentLine({
        operationId: 'op',
        date: '2026-09-06',
        amountKopecks: 1,
        daysUntil: 1,
      }),
    ).toBe('1 день до следующего платежа');
  });

  it('день платежа — сегодня', () => {
    expect(
      rentalNextPaymentLine({
        operationId: 'op',
        date: '2026-09-05',
        amountKopecks: 1,
        daysUntil: 0,
      }),
    ).toBe('Платёж сегодня');
  });
});

describe('rentalOverdueLine', () => {
  it('красная строка просрочки со склонением (#817: «Просрочен 1 месяц» / «Просрочено 2 месяца»)', () => {
    expect(rentalOverdueLine(1)).toBe('Просрочен 1 месяц');
    expect(rentalOverdueLine(2)).toBe('Просрочено 2 месяца');
    expect(rentalOverdueLine(5)).toBe('Просрочено 5 месяцев');
    expect(rentalOverdueLine(11)).toBe('Просрочено 11 месяцев');
    expect(rentalOverdueLine(21)).toBe('Просрочен 21 месяц');
    expect(rentalOverdueLine(22)).toBe('Просрочено 22 месяца');
    expect(rentalOverdueLine(101)).toBe('Просрочен 101 месяц');
    expect(rentalOverdueLine(111)).toBe('Просрочено 111 месяцев');
  });
});

describe('rentalRemainingLine', () => {
  it('остаток месяцев со склонением', () => {
    expect(rentalRemainingLine(23)).toBe('Осталось 23 месяца аренды');
    expect(rentalRemainingLine(1)).toBe('Остался 1 месяц аренды');
    expect(rentalRemainingLine(5)).toBe('Осталось 5 месяцев аренды');
    expect(rentalRemainingLine(21)).toBe('Остался 21 месяц аренды');
    expect(rentalRemainingLine(11)).toBe('Осталось 11 месяцев аренды');
    expect(rentalRemainingLine(101)).toBe('Остался 101 месяц аренды');
    expect(rentalRemainingLine(111)).toBe('Осталось 111 месяцев аренды');
  });

  it('бессрочная аренда — строки остатка нет', () => {
    expect(rentalRemainingLine(null)).toBeUndefined();
  });
});

describe('rentAmountPerMonth', () => {
  it('канонический money-формат с периодом', () => {
    expect(rentAmountPerMonth(5_600_000)).toBe(`${formatMoneyKopecks(5_600_000)} в месяц`);
  });
});

describe('rentalTeaserRows', () => {
  it('три строки карточки детализации: плата, день, начало', () => {
    expect(rentalTeaserRows(rentalFixture())).toEqual([
      { label: 'Арендная плата', value: `${formatMoneyKopecks(5_600_000)} в месяц` },
      { label: 'День оплаты', value: '10 число' },
      { label: 'Начало аренды', value: '10 октября' },
    ]);
  });

  it('последний день месяца и день оплаты «last»', () => {
    const rental = rentalFixture({
      rentPayment: {
        paymentId: '0198f6a1-7c1a-7d0f-9f4f-6f3c1e2b4a88',
        amountKopecks: 5_600_000,
        paymentDay: 'last',
        autoPay: false,
        reminderOffsetDays: null,
        nextPayment: null,
      },
    });
    expect(rentalTeaserRows(rental)).toContainEqual({
      label: 'День оплаты',
      value: 'Последний день месяца',
    });
  });
});

describe('rentalTermsRows', () => {
  it('полный экран условий заполненной аренды (1302:53783)', () => {
    const rows = rentalTermsRows(rentalFixture(), '2026-09-05');
    expect(rows).toEqual([
      { label: 'Арендная плата', value: `${formatMoneyKopecks(5_600_000)} в месяц` },
      { label: 'Залог', value: `${formatMoneyKopecks(5_600_000)}` },
      { label: 'Комиссия', value: `${formatMoneyKopecks(0)}` },
      { label: 'День оплаты', value: '10 число' },
      { label: 'Срок аренды', value: '24 месяца' },
      { label: 'Коммунальные платежи', value: 'Только счетчики' },
      { label: 'Начало аренды', value: '10 октября' },
      { label: 'Окончание аренды', value: '10 октября, 2028' },
    ]);
  });

  it('пустые значения (1550:94419): деньги — «0 ₽», строки — «Не указано»', () => {
    const rows = rentalTermsRows(
      rentalFixture({
        plannedEndDate: null,
        depositKopecks: null,
        commissionKopecks: null,
        progress: { paidMonths: 6, totalMonths: null, monthsRemaining: null, overdueMonths: null },
        comment: '',
      }),
      '2026-09-05',
    );
    expect(rows).toEqual([
      { label: 'Арендная плата', value: `${formatMoneyKopecks(5_600_000)} в месяц` },
      { label: 'Залог', value: formatMoneyKopecks(0) },
      { label: 'Комиссия', value: formatMoneyKopecks(0) },
      { label: 'День оплаты', value: '10 число' },
      { label: 'Срок аренды', value: 'Не указано' },
      { label: 'Коммунальные платежи', value: 'Только счетчики' },
      { label: 'Начало аренды', value: '10 октября' },
      { label: 'Окончание аренды', value: 'Не указано' },
    ]);
  });

  it('начало и окончание в текущем году — без года', () => {
    const rows = rentalTermsRows(
      rentalFixture({
        startDate: '2026-05-10',
        plannedEndDate: '2026-11-10',
        progress: { paidMonths: 1, totalMonths: 6, monthsRemaining: 2, overdueMonths: null },
      }),
      '2026-09-05',
    );
    expect(rows).toContainEqual({ label: 'Начало аренды', value: '10 мая' });
    expect(rows).toContainEqual({ label: 'Окончание аренды', value: '10 ноября' });
  });
});

describe('rentalCommentText', () => {
  it('пустой комментарий — «Не указано», непустой — текст как есть', async () => {
    const { rentalCommentText } = await import('./rental-view');
    expect(rentalCommentText('Дом — панельный')).toBe('Дом — панельный');
    expect(rentalCommentText('   ')).toBe('Не указано');
  });
});

describe('rentalElapsedLine', () => {
  it('бессрочная: первый месяц идёт, полных месяцев нет', () => {
    expect(rentalElapsedLine('2026-09-05', '2026-09-07')).toBe('Идёт 1 месяц');
    expect(rentalElapsedLine('2026-09-07', '2026-09-07')).toBe('Идёт 1 месяц');
  });

  it('бессрочная: прошедшие месяцы со склонением (решение владельца 2026-09-07)', () => {
    expect(rentalElapsedLine('2025-09-07', '2026-09-07')).toBe('Прошло 12 месяцев');
    expect(rentalElapsedLine('2026-08-07', '2026-09-07')).toBe('Прошёл 1 месяц');
    expect(rentalElapsedLine('2026-07-07', '2026-09-07')).toBe('Прошло 2 месяца');
    expect(rentalElapsedLine('2026-04-10', '2026-09-07')).toBe('Прошло 4 месяца');
    expect(rentalElapsedLine('2025-10-07', '2026-09-07')).toBe('Прошло 11 месяцев');
    expect(rentalElapsedLine('2024-12-07', '2026-09-07')).toBe('Прошёл 21 месяц');
    expect(rentalElapsedLine('2024-11-07', '2026-09-07')).toBe('Прошло 22 месяца');
  });
});

describe('rentalTenantTitle', () => {
  it('полное имя арендатора', () => {
    expect(
      rentalTenantTitle({
        contactId: 'c',
        firstName: 'Александр',
        lastName: 'Петров',
        phone: '+79934302010',
      }),
    ).toBe('Александр Петров');
  });
});

describe('hasProgressCard', () => {
  it('со следующим платежом карточка видима (1232:61259)', () => {
    const rental = rentalFixture();
    expect(hasProgressCard(rental.rentPayment.nextPayment, rental.progress)).toBe(true);
  });

  it('F1: день планового окончания и «Ожидает действия» — карточка скрыта целиком', () => {
    const rental = rentalFixture({
      progress: { paidMonths: 6, totalMonths: 24, monthsRemaining: 0, overdueMonths: null },
    });
    expect(hasProgressCard(null, rental.progress)).toBe(false);
  });

  it('#817: просрочка — содержимое карточки: жива и без синей строки и остатка', () => {
    const rental = rentalFixture({
      progress: { paidMonths: 4, totalMonths: 24, monthsRemaining: 0, overdueMonths: 2 },
    });
    expect(hasProgressCard(null, rental.progress)).toBe(true);
  });

  it('без платежа, но с остатком — карточка остаётся ради строки «Осталось N»', () => {
    const rental = rentalFixture({
      progress: { paidMonths: 6, totalMonths: 24, monthsRemaining: 3, overdueMonths: null },
    });
    expect(hasProgressCard(null, rental.progress)).toBe(true);
  });

  it('бессрочная без платежа — карточка остаётся ради «Прошло N месяцев»', () => {
    const rental = rentalFixture({
      progress: { paidMonths: 0, totalMonths: null, monthsRemaining: null, overdueMonths: null },
    });
    expect(hasProgressCard(null, rental.progress)).toBe(true);
  });
});
