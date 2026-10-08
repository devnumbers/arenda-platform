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

  it('ключ операции — факт оплаты; плановой строки в paid-скоупе нет', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ date: '2026-09-05', paidDate: '2026-09-01' })],
      [],
      today,
    );

    expect(groups[0]?.date).toBe('2026-09-01');
  });

  it('день правки — локальная дата created_at, не UTC-дата', () => {
    // Полдень 1 сентября локально — UTC-момент, чей UTC-дата в поясах
    // восточнее UTC+12 уже 2-е: фикстура через localNoonIso держит дату
    // стабильной, проверяем что ключ — локальный день.
    const groups = buildPaymentHistoryTimeline(
      [],
      [change('ch-1', localMomentIso(2026, 9, 1))],
      today,
    );

    expect(groups).toHaveLength(1);
    expect(groups[0]?.date).toBe('2026-09-01');
  });
});

describe('buildPaymentHistoryTimeline — порядок внутри дня', () => {
  const today: IsoDate = '2026-09-10';

  it('операции дня выше правок дня: строка операции остаётся на месте дефолтного режима', () => {
    const groups = buildPaymentHistoryTimeline(
      [operation({ paidDate: '2026-09-01' })],
      [
        change('ch-morning', localMomentIso(2026, 9, 1, 9)),
        change('ch-evening', localMomentIso(2026, 9, 1, 18)),
      ],
      today,
    );

    const items = groups[0]?.items ?? [];
    expect(items).toHaveLength(3);
    expect(items[0]?.kind).toBe('operation');
    // Правки дня — по убыванию created_at под операцией.
    expect(items[1]?.kind).toBe('changes');
    expect(items[1] && items[1].kind === 'changes' && items[1].entry.id).toBe('ch-evening');
    expect(items[2]?.kind).toBe('changes');
    expect(items[2] && items[2].kind === 'changes' && items[2].entry.id).toBe('ch-morning');
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
