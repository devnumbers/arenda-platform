import type {
  IsoDate,
  PaymentChangeEntry,
  PaymentOperation,
} from '@/entities/payment';
import { addDays, dateToIso, dateToIsoLocal, fromIso } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { paymentChangeChips } from './payment-change-chips';

/**
 * Лента «История платежа» в режиме изменений (макеты 3214-73216/73857,
 * тикет #1195): чипы правок вперемешку с операциями, группы по дням —
 * «Сегодня»/«Вчера»/дата (год вне текущего). День строки журнала —
 * локальная дата created_at; день операции — факт оплаты (канон историй,
 * #994). Внутри дня операции выше правок: строка операции остаётся на
 * месте дефолтного режима, правки дня (со временем) встают под неё;
 * порядок правок между собой — по (created_at, id) DESC, ключ keyset
 * сервера (канон #597, UUIDv7).
 */

/** Элемент группы дня: строка операции или блок чипов одной строки
 * журнала. Чипы предрасчитаны моделью чипов — экран текст не собирает. */
export type PaymentHistoryTimelineItem =
  | { readonly kind: 'operation'; readonly operation: PaymentOperation }
  | {
      readonly kind: 'changes';
      readonly entry: PaymentChangeEntry;
      readonly chips: ReadonlyArray<string>;
    };

export type PaymentHistoryTimelineGroup = {
  /** Дата группы ('YYYY-MM-DD') — стабильный ключ секции списка. */
  readonly date: IsoDate;
  readonly label: string;
  readonly items: ReadonlyArray<PaymentHistoryTimelineItem>;
};

/** Число миллисекунд от начала суток — операция не несёт времени, ключ
 * сортировки считает её концом дня: в обратной хронологии день операции
 * открывается строкой операции, правки этого дня идут под ней. */
const DAY_END_MS = 24 * 60 * 60 * 1000 - 1;

export function buildPaymentHistoryTimeline(
  operations: ReadonlyArray<PaymentOperation>,
  changes: ReadonlyArray<PaymentChangeEntry>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryTimelineGroup> {
  type TimelineEntry = {
    readonly date: IsoDate;
    readonly sortMs: number;
    readonly item: PaymentHistoryTimelineItem;
  };

  const entries: TimelineEntry[] = [];

  for (const operation of operations) {
    const date = operation.paidDate ?? operation.date;
    entries.push({
      date,
      // Конец дня в UTC-шкале канона (fromIso): «операция случилась в этот
      // день» — ниже любых правок своего дня, выше правок предыдущего.
      sortMs: fromIso(date).getTime() + DAY_END_MS,
      item: { kind: 'operation', operation },
    });
  }

  for (const entry of changes) {
    const chips = paymentChangeChips(entry);
    // Строки без содержимого (updated без дифа) ленту не засоряют:
    // paused/resumed всегда дают свой чип.
    if (chips.length === 0) {
      continue;
    }
    const moment = Date.parse(entry.createdAt);
    const date = dateToIsoLocal(new Date(moment));
    entries.push({
      date,
      // Ключ — в шкале дня группы: время суток момента (смещение от
      // полуночи его UTC-дня) поверх полуночи локального дня. Сырой момент
      // ломал бы grouping на краю суток: правка первых часов локального дня
      // (00:00–03:00 МСК — UTC-момент предыдущего дня) сортировалась бы
      // ниже операций предыдущего дня, и дни шли бы D, D−1, D — «Сегодня»
      // дважды и дубли ключей секций.
      sortMs: fromIso(date).getTime() + (moment - fromIso(dateToIso(new Date(moment))).getTime()),
      item: { kind: 'changes', entry, chips },
    });
  }

  entries.sort((a, b) => {
    if (a.sortMs !== b.sortMs) {
      return b.sortMs - a.sortMs;
    }
    // Равные моменты: операции дня выше уже покрыты концом дня; для строк
    // журнала — tie по id (UUIDv7 монотонен, канон #597).
    if (a.item.kind === 'changes' && b.item.kind === 'changes') {
      const aId = a.item.entry.id;
      const bId = b.item.entry.id;
      return bId < aId ? -1 : bId > aId ? 1 : 0;
    }
    return 0;
  });

  const groups: { label: string; date: IsoDate; items: PaymentHistoryTimelineItem[] }[] = [];
  for (const entry of entries) {
    const current = groups.at(-1);
    if (current !== undefined && current.date === entry.date) {
      current.items.push(entry.item);
      continue;
    }
    groups.push({
      label: timelineGroupLabel(entry.date, today),
      date: entry.date,
      items: [entry.item],
    });
  }
  return groups;
}

/** Лейблы дня макета 3214-73216: «Сегодня»/«Вчера» без даты (канон
 * историй 1302:52209), дальше день-месяц; год — только вне текущего
 * («25 декабря, 2025»), в текущем «1 сентября» (макет). */
function timelineGroupLabel(date: IsoDate, today: IsoDate): string {
  if (date === today) {
    return 'Сегодня';
  }
  if (date === addDays(today, -1)) {
    return 'Вчера';
  }
  return formatDayMonthWithYear(date, today);
}
