import type {
  IsoDate,
  PaymentChangeEntry,
  PaymentOperation,
} from '@/entities/payment';
import { addDays, cmp, dateToIsoLocal, fromIso } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { paymentChangeChips } from './payment-change-chips';

/**
 * Лента «История платежа» (макеты 3214-73216/73857, тикет #1195): чипы
 * правок вперемешку с операциями строго по реальному времени, группы по
 * дням — «Сегодня»/«Вчера»/дата (год вне текущего). День группы — день
 * факта оплаты (paid_date, #994) у операции и локальная дата created_at у
 * правки. Внутри дня каждая строка стоит на своём моменте: у операции это
 * updatedAt — момент UPDATE оплаты (оплаченная строка после оплаты не
 * меняется, #1195), у правки — created_at; поэтому правки встают и выше, и
 * ниже оплаты своего дня (макет «1 сентября»), а последняя оплата открывает
 * день. Порядок правок между собой при равных моментах — по id DESC
 * (UUIDv7 монотонен, канон #597).
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

/** Ключ сортировки строки ленты: день группы (лексикографическое сравнение
 * ISO-дат) и момент внутри дня (абсолютное время — у обоих видов строк
 * одинаковая шкала, никакой подгонки «конца дня» — именно она ставила
 * правки после оплаты под оплатой, #1195). */
type TimelineEntry = {
  readonly date: IsoDate;
  readonly moment: number;
  readonly item: PaymentHistoryTimelineItem;
};

export function buildPaymentHistoryTimeline(
  operations: ReadonlyArray<PaymentOperation>,
  changes: ReadonlyArray<PaymentChangeEntry>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryTimelineGroup> {
  const entries: TimelineEntry[] = [];

  for (const operation of operations) {
    const date = operation.paidDate ?? operation.date;
    const paid = Date.parse(operation.updatedAt ?? '');
    entries.push({
      date,
      // Строка без времени (старый ответ) садится на начало своего дня —
      // правки дня закономерно оказываются выше.
      moment: Number.isNaN(paid) ? fromIso(date).getTime() : paid,
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
    entries.push({
      date: dateToIsoLocal(new Date(moment)),
      moment,
      item: { kind: 'changes', entry, chips },
    });
  }

  entries.sort((a, b) => {
    if (a.date !== b.date) {
      // Обратная хронология дней; сами дни не зависят от моментов, поэтому
      // группы убывают строго (дублей «Сегодня» не бывает на краю суток).
      return cmp(b.date, a.date);
    }
    if (a.moment !== b.moment) {
      return b.moment - a.moment;
    }
    // Равные моменты: строки журнала — tie по id (канон #597).
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
