/**
 * Группировка ленты «История действий» (тикет #709, макет 2157-56786):
 * «дата → объект → актёр → строки» (ADR 0061 §6). Лента приходит по
 * возрастанию времени (новые снизу, как в мессенджере); канон группировки
 * #624 — подряд идущие записи одного дня образуют одну секцию. Чипы дней —
 * «Сегодня»/«Вчера»/дальняя дата (канон дат shared/lib); объекты и актёры
 * внутри дня — подряд идущие серии, как в мессенджере: строгая хронология
 * дня не переупорядочивается, вернувшийся объект/актёр открывает новую
 * серию. Актёр — шапка группы, в тексте строки его нет (ADR 0061 §6);
 * обезличенные записи (пользователь удалён, actor_id null) группируются
 * по снимку имени. День чипа «Сегодня»/«Вчера» — локальный календарный
 * день смотрящего (dateToIsoLocal), тогда как период-фильтр ленты сервер
 * матчит как UTC-сутки (канон админ-аудита): расхождение ограничено
 * краевыми часами суток и принято — прецедент клиентского «сегодня»
 * проекции платежей #453, ADR 0061 §7.
 */

import type { HistoryEntry } from '@/entities/history';
import { addDays, dateToIsoLocal, type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';

export type HistoryActorGroup = {
  /** actor_id снимка записи; null — пользователь удалён, запись
   * обезличена (шапка такой группы не кликабельна). */
  readonly actorId: string | null;
  /** actor_id, обезличенным записям — ключ по снимку имени. */
  readonly key: string;
  readonly name: string;
  readonly entries: readonly HistoryEntry[];
};

export type HistoryObjectGroup = {
  readonly propertyId: string;
  readonly propertyName: string;
  readonly actors: readonly HistoryActorGroup[];
};

export type HistoryDayGroup = {
  /** Локальный календарный день группы ('YYYY-MM-DD') — ключ секции. */
  readonly day: IsoDate;
  readonly label: string;
  readonly objects: readonly HistoryObjectGroup[];
};

type MutableActorGroup = {
  actorId: string | null;
  key: string;
  name: string;
  entries: HistoryEntry[];
};
type MutableObjectGroup = {
  propertyId: string;
  propertyName: string;
  actors: MutableActorGroup[];
};
type MutableDayGroup = {
  day: IsoDate;
  label: string;
  objects: MutableObjectGroup[];
};

export function groupHistoryByDay(
  entries: ReadonlyArray<HistoryEntry>,
  today: IsoDate,
): ReadonlyArray<HistoryDayGroup> {
  const yesterday = addDays(today, -1);

  const days: MutableDayGroup[] = [];
  for (const entry of entries) {
    const day = dateToIsoLocal(new Date(entry.createdAt));
    let current = days.at(-1);
    if (current === undefined || current.day !== day) {
      current = { day, label: dayLabel(day, today, yesterday), objects: [] };
      days.push(current);
    }

    // Объекты и актёры — идущие подряд серии (канон мессенджера): строгая
    // хронология дня не переупорядочивается, вернувшийся актёр открывает
    // новую серию.
    let object_ = current.objects.at(-1);
    if (object_ === undefined || object_.propertyId !== entry.propertyId) {
      object_ = { propertyId: entry.propertyId, propertyName: entry.propertyName, actors: [] };
      current.objects.push(object_);
    }

    const actorKey = entry.actorId ?? `~deleted:${entry.actorName}`;
    let actor = object_.actors.at(-1);
    if (actor === undefined || actor.key !== actorKey) {
      actor = {
        actorId: entry.actorId,
        key: actorKey,
        name: entry.actorName,
        entries: [],
      };
      object_.actors.push(actor);
    }
    actor.entries.push(entry);
  }
  return days;
}

function dayLabel(day: IsoDate, today: IsoDate, yesterday: IsoDate): string {
  if (day === today) return 'Сегодня';
  if (day === yesterday) return 'Вчера';
  return formatDayMonthWithYear(day, today);
}
