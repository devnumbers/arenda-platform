import { describe, expect, it } from 'vitest';
import type {
  IsoDate,
  PaymentChangeEntry,
  PaymentFieldChange,
  PaymentOperation,
} from '@/entities/payment';
import { buildPaymentHistoryTimeline } from './payment-history-timeline';

/** Момент действия локальным полуднём заданной даты (час — сценарный):
 * ISO-строка от локальных компонентов — дата-ключ строки журнала стабилен
 * в любой TZ машины (9–18 часов не переходят через сутки ни в одном поясе;
 * браузер стенда — UTC, канон e2e-фикстур). */
function localMomentIso(year: number, month1: number, day: number, hour = 12): string {
  return new Date(year, month1 - 1, day, hour, 0).toISOString();
}

function operation(overrides: Partial<PaymentOperation> = {}): PaymentOperation {
  return {
    id: 'op-1',
    propertyId: 'p-1',
    paymentId: 'pay-1',
    date: '2026-09-01',
    paidDate: '2026-09-01',
    status: 'paid',
    type: 'expense',
    title: 'Юрист',
    amountKopecks: 250_000,
    categoryLabel: 'Юридические услуги',
    updatedAt: '2026-09-01T12:00:00Z',
    ...overrides,
  };
}

/** Правка одним полем; сценарные строки — вторым аргументом. */
function change(
  id: string,
  createdAt: string,
  changes: ReadonlyArray<PaymentFieldChange> = [
    { field: 'amount_kopecks', old: 250_000, new: 180_000 },
  ],
  action: PaymentChangeEntry['action'] = 'updated',
): PaymentChangeEntry {
  return { id, action, changes, createdAt };
}

describe('buildPaymentHistoryTimeline — группы по датам (макет 3214-73216)', () => {
  const today: IsoDate = '2026-09-10';

  it('сливает операции и правки в обратной хронологии: день операции выше дней правок', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ paidDate: '2026-09-01' })],
      [
        change('ch-old', localMomentIso(2026, 8, 15)),
        change('ch-new', localMomentIso(2026, 9, 1, )),
      ],
      today,
    );

    expect(groups.map((group) => group.date)).toEqual(['2026-09-01', '2026-08-15']);
  });

  it('лейблы групп: «Сегодня», «Вчера», дата без года в текущем (макет «1 сентября»)', () => {
    const groups = buildPaymentHistoryTimeline(
      [
        operation({ paidDate: '2026-09-10' }),
        operation({ paidDate: '2026-09-09' }),
        operation({ paidDate: '2026-09-01' }),
      ],
      [change('ch-old', localMomentIso(2025, 12, 25))],
      today,
    );

    expect(groups.map((group) => group.label)).toEqual([
      'Сегодня',
      'Вчера',
      '1 сентября',
      '25 декабря, 2025',
    ]);
  });

  it('ключ операции — факт оплаты (день реальной оплаты, решение владельца #1195); плановой строки в paid-скоупе нет', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ date: '2026-09-05', paidDate: '2026-09-01' })],
      [],
      today,
    );

    expect(groups[0]?.date).toBe('2026-09-01');
  });

  it('день правки — локальная дата created_at, не UTC-дата', () => {
    // Полдень 1 сентября локально — UTC-момент, чей UTC-дата в поясах
    // восточнее UTC+12 уже 2-е: фикстура через localMomentIso держит дату
    // стабильной, проверяем что ключ — локальный день.
    const groups = buildPaymentHistoryTimeline(
      [],
      [change('ch-1', localMomentIso(2026, 9, 1))],
      today,
    );

    expect(groups).toHaveLength(1);
    expect(groups[0]?.date).toBe('2026-09-01');
  });

  it('правка первых часов локального дня не ломает монотонность групп (край суток)', () => {
    // Момент 21:00Z: по UTC это день 1-го, в поясах UTC+3 и восточнее —
    // уже день 2-го. День группы и момент — независимые ключи (дни
    // сравниваются сами по себе), так что группы строго убывают и
    // уникальны в любой TZ машины: «Сегодня» не может отрендериться дважды.
    const groups = buildPaymentHistoryTimeline(
      [
        operation({ paidDate: '2026-09-01', updatedAt: '2026-09-01T12:00:00Z' }),
        operation({ paidDate: '2026-09-02', updatedAt: '2026-09-02T12:00:00Z' }),
      ],
      [change('ch-night', '2026-09-01T21:00:00Z')],
      today,
    );

    const dates = groups.map((group) => group.date);
    expect(dates).toEqual([...dates].sort().reverse());
    expect(new Set(dates).size).toBe(dates.length);
  });
});

describe('buildPaymentHistoryTimeline — внутри дня строго по времени (#1195)', () => {
  const today: IsoDate = '2026-09-10';

  it('правки и операция вперемешку по реальным моментам: чипы и выше, и ниже оплаты (макет «1 сентября»)', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ paidDate: '2026-09-01', updatedAt: '2026-09-01T11:00:00Z' })],
      [
        change('ch-morning', '2026-09-01T08:00:00Z'),
        change('ch-evening', '2026-09-01T18:00:00Z'),
      ],
      today,
    );

    const kinds = (groups[0]?.items ?? []).map((item) => item.kind);
    expect(kinds).toEqual(['changes', 'operation', 'changes']);
  });

  it('последняя оплаченная операция открывает свой день (баг владельца #1195)', () => {
    const groups = buildPaymentHistoryTimeline(
      [
        operation({ id: 'op-first-pay', paidDate: '2026-09-01', updatedAt: '2026-09-01T09:00:00Z' }),
        operation({ id: 'op-last-pay', paidDate: '2026-09-01', updatedAt: '2026-09-01T16:00:00Z' }),
      ],
      [],
      today,
    );

    const ids = (groups[0]?.items ?? []).map((item) =>
      item.kind === 'operation' ? item.operation.id : '',
    );
    expect(ids).toEqual(['op-last-pay', 'op-first-pay']);
  });

  it('день группы — дата факта, момент внутри дня — время оплаты: оплата старого дня не уезжает наверх ленты', () => {
    // Аномалия данных (в проде оплата штампует paid_date днём оплаты),
    // модель обязана держать группы по датам монотонными.
    const groups = buildPaymentHistoryTimeline(
      [
        operation({ paidDate: '2026-09-01', updatedAt: '2026-09-09T12:00:00Z' }),
        operation({ paidDate: '2026-09-08', updatedAt: '2026-09-08T12:00:00Z' }),
      ],
      [],
      today,
    );

    expect(groups.map((group) => group.date)).toEqual(['2026-09-08', '2026-09-01']);
  });

  it('момент оплаты берётся из updatedAt строки, а не из конца дня', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ paidDate: '2026-09-01', updatedAt: '2026-09-01T15:00:00Z' })],
      [change('ch-after-pay', '2026-09-01T18:00:00Z')],
      today,
    );

    // Правка позже оплаты — выше неё, хоть у операции и «вся дата».
    expect(groups[0]?.items[0]?.kind).toBe('changes');
  });

  it('без updatedAt (старый ответ) операция садится на начало своего дня: правки дня выше', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ paidDate: '2026-09-01', updatedAt: '' })],
      [change('ch-day', '2026-09-01T10:00:00Z')],
      today,
    );

    expect(groups[0]?.items[0]?.kind).toBe('changes');
  });

  it('чипы правки предрасчитаны моделью чипов и едут в элементе', () => {
    const groups = buildPaymentHistoryTimeline(
      [],
      [
        change('ch-1', localMomentIso(2026, 9, 1), [
          { field: 'title', old: 'Юрист', new: 'Документы' },
        ]),
      ],
      today,
    );

    const item = groups[0]?.items[0];
    expect(item?.kind).toBe('changes');
    if (item?.kind === 'changes') {
      expect(item.chips).toEqual(['Название изменено: «Документы»']);
    }
  });

  it('строки правок с пустым дифом (updated без полей) не рисуются', () => {
    const groups = buildPaymentHistoryTimeline(
      [],
      [change('ch-empty', localMomentIso(2026, 9, 1), [])],
      today,
    );

    expect(groups).toEqual([]);
  });

  it('пауза и возобновление — строки журнала с одним чипом', () => {
    const groups = buildPaymentHistoryTimeline(
      [],
      [
        change('ch-pause', localMomentIso(2026, 9, 2), [], 'paused'),
        change('ch-resume', localMomentIso(2026, 9, 1), [], 'resumed'),
      ],
      today,
    );

    expect(groups).toHaveLength(2);
    const pause = groups[0]?.items[0];
    if (pause?.kind === 'changes') {
      expect(pause.chips).toEqual(['Платеж поставлен на паузу']);
    }
  });

  it('устойчивость к равным моментам правок: tie по id (UUIDv7, канон #597)', () => {
    const sameMoment = localMomentIso(2026, 9, 1);
    const groups = buildPaymentHistoryTimeline(
      [],
      [change('ch-a', sameMoment), change('ch-b', sameMoment)],
      today,
    );

    const ids = (groups[0]?.items ?? []).map((item) =>
      item.kind === 'changes' ? item.entry.id : '',
    );
    expect(ids).toEqual(['ch-b', 'ch-a']);
  });
});
