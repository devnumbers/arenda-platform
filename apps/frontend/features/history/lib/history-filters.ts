import type { IsoDate, IsoRange } from '@/shared/lib/calendar';
import type { HistoryBaseAction, HistoryKind, HistoryParticipantOption } from '@/entities/history';
import { HISTORY_KINDS } from '@/entities/history';
import { formatIsoRangeChipLabel } from '@/shared/lib/date-format';
import { readIsoRangeParam, type UrlParamsSource } from '@/shared/lib/parse-iso-range-param';
import type { HistoryFeedScope } from '@/shared/api/query-keys';

/**
 * Фильтры ленты «Истории действий» (#711, макеты 2177-60527,
 * 2067-163528/162950, 2050-158280): период, основные действия, виды,
 * участники, объекты. Состояние живёт в адресе ленты (канон «Состояние
 * страницы в адресе», DESIGN.md §3: ?from=&to=&actions=&kinds=&actors=
 * &objects=), «Применить фильтры» пишет push — «назад» возвращает без
 * фильтров. Семантика группы (аннотация макета: «открывается со
 * свернутыми и всеми выбранными категориями»): null — «все» (параметра
 * нет, сервер не фильтрует), список — только выбранное, пустой список —
 * «ни один» (пустое значение параметра; результат ленты пуст, запрос не
 * делается — серверный контракт пустой список не отличает от «все»).
 */

export type HistoryFilters = {
  /** Диапазон периода; null — весь период. */
  readonly period: IsoRange | null;
  /** Выбранные основные действия; null — все. */
  readonly actions: ReadonlyArray<HistoryBaseAction> | null;
  /** Выбранные виды действий; null — все. */
  readonly kinds: ReadonlyArray<HistoryKind> | null;
  /** Выбранные участники (uuid); null — все. */
  readonly actorIds: ReadonlyArray<string> | null;
  /** Выбранные объекты (uuid); null — все. */
  readonly propertyIds: ReadonlyArray<string> | null;
};

/** Дефолт: весь период, все группы «все» (макет 2177-60527). */
export const DEFAULT_HISTORY_FILTERS: HistoryFilters = {
  period: null,
  actions: null,
  kinds: null,
  actorIds: null,
  propertyIds: null,
};

/**
 * Минимальный источник параметров — структурный тип UrlParamsSource из
 * канона parse-iso-range-param; отдельное имя оставлено для читаемости
 * сигнатур этого модуля.
 */
export type HistoryParamsSource = UrlParamsSource;

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const BASE_ACTIONS: ReadonlyArray<HistoryBaseAction> = ['added', 'changed', 'completed', 'deleted'];
const KIND_SET = new Set<string>(HISTORY_KINDS);

/**
 * CSV-группа: пустое значение — «ни один» ([]), отсутствующее — «все»
 * (null); элементы чистятся (валидные слаги/uuid, без дублей, порядок
 * сохранён). Неизвестные значения отбрасываются — ссылка с мусором читается
 * как ближайшее осмысленное состояние, а не ломает ленту.
 */
function readCsvGroup<T extends string>(
  raw: string | null,
  isValid: (value: string) => value is T,
): ReadonlyArray<T> | null {
  if (raw === null) {
    return null;
  }
  const result: T[] = [];
  const seen = new Set<string>();
  for (const part of raw.split(',')) {
    const value = part.trim();
    if (value.length === 0 || seen.has(value) || !isValid(value)) {
      continue;
    }
    seen.add(value);
    result.push(value);
  }
  return result;
}

/** Чтение фильтров из адреса: период — канон readIsoRangeParam (правила
 * #477: битые даты, перевёрнутый период отбрасываются; будущий хвост
 * обрезается «сегодня»), CSV-группы — чистка валидных значений без дублей. */
export function readHistoryFilters(params: HistoryParamsSource, today: IsoDate): HistoryFilters {
  return {
    period: readIsoRangeParam(params, 'from', 'to', today),
    actions: readCsvGroup(params.get('actions'), (value): value is HistoryBaseAction =>
      BASE_ACTIONS.includes(value as HistoryBaseAction),
    ),
    kinds: readCsvGroup(params.get('kinds'), (value): value is HistoryKind => KIND_SET.has(value)),
    actorIds: readCsvGroup(params.get('actors'), (value): value is string => UUID_RE.test(value)),
    propertyIds: readCsvGroup(params.get('objects'), (value): value is string => UUID_RE.test(value)),
  };
}

/**
 * Patch адреса (канон: дефолтные значения не пишутся): «все» (null) — ключ
 * отсутствует, «ни один» — пустое значение (отличимо от отсутствия), выбор
 * — comma-list. from/to пишутся парой всегда, когда период применён.
 */
export function historyFiltersParams(filters: HistoryFilters): Record<string, string> {
  const result: Record<string, string> = {};
  if (filters.period !== null) {
    result.from = filters.period.from;
    result.to = filters.period.to;
  }
  if (filters.actions !== null) {
    result.actions = filters.actions.join(',');
  }
  if (filters.kinds !== null) {
    result.kinds = filters.kinds.join(',');
  }
  if (filters.actorIds !== null) {
    result.actors = filters.actorIds.join(',');
  }
  if (filters.propertyIds !== null) {
    result.objects = filters.propertyIds.join(',');
  }
  return result;
}

/** Дефолт ли: весь период и все группы «все». */
export function isDefaultHistoryFilters(filters: HistoryFilters): boolean {
  return (
    filters.period === null &&
    filters.actions === null &&
    filters.kinds === null &&
    filters.actorIds === null &&
    filters.propertyIds === null
  );
}

/**
 * Скоуп ленты GET /history (#708) из фильтров; null — какая-то из групп
 * выбрана «в ноль»: по семантике фильтра результат пуст, а серверный
 * контракт пустой список не отличает от «все» — лента не запрашивается
 * вовсе (экран рисует пустое состояние без запроса).
 */
export function historyFeedScope(filters: HistoryFilters): HistoryFeedScope | null {
  if (filters.actions !== null && filters.actions.length === 0) {
    return null;
  }
  if (filters.kinds !== null && filters.kinds.length === 0) {
    return null;
  }
  if (filters.actorIds !== null && filters.actorIds.length === 0) {
    return null;
  }
  if (filters.propertyIds !== null && filters.propertyIds.length === 0) {
    return null;
  }
  return {
    dateFrom: filters.period?.from,
    dateTo: filters.period?.to,
    actions: filters.actions ?? undefined,
    kinds: filters.kinds ?? undefined,
    actorIds: filters.actorIds ?? undefined,
    propertyIds: filters.propertyIds ?? undefined,
  };
}

/**
 * Фильтры адреса на странице «Действия участника» (#712): человек прибит
 * путём страницы и фильтром «Участники» не является — группа не читается
 * вовсе (в шите её нет; hand-crafted ?actors= в адресе игнорируется, а
 * «Применить фильтры» его и вычищает — запись группы своя).
 */
export function memberHistoryFilters(filters: HistoryFilters): HistoryFilters {
  return { ...filters, actorIds: null };
}

/**
 * Скоуп «Действий участника» (#712, ADR 0061 §7 — тот же GET /history,
 * actor_ids = один): группы адреса действуют поверх прибитого актёра;
 * null — как у historyFeedScope, какая-то из групп адреса выбрана «в
 * ноль». Группа «Участники» адреса не действует никогда — человек прибит
 * страницей (memberHistoryFilters).
 */
export function historyMemberFeedScope(
  filters: HistoryFilters,
  participantId: string,
): HistoryFeedScope | null {
  const base = historyFeedScope(memberHistoryFilters(filters));
  return base !== null ? { ...base, actorIds: [participantId] } : null;
}

/** Лейбл чипа периода шита: без периода — «Выбрать период» (макет
 * 2177-60527), с периодом — формат чипа канона (макет 2067-162950). */
export function historyPeriodChipLabel(period: IsoRange | null): string {
  return period !== null ? formatIsoRangeChipLabel(period) : 'Выбрать период';
}

/** Заголовок строки участника шита: чужой — канон отображаемого имени,
 * себе — только имя без фамилии (макет 2067-163528: «Даниил (Вы)»;
 * суффикс «(Вы)» серым рисует сам шит); имени нет — канон (маскированный
 * телефон). */
export function historyParticipantTitle(
  participant: Pick<HistoryParticipantOption, 'name' | 'firstName'>,
  isMe: boolean,
): string {
  if (!isMe) {
    return participant.name;
  }
  return participant.firstName !== '' ? participant.firstName : participant.name;
}

/** Переключение группы мастер-чекбоксом шита: «все» (null) — снять всю
 * группу ([]); иначе (частичный выбор или «ни одного») — поставить всю
 * группу (null). Решение владельца 23.09: тап по мастер-чекбоксу из
 * состояния «все» должен снимать группу, а не быть но-опом. */
export function toggleHistoryFilterGroup(
  current: ReadonlyArray<string> | null,
): ReadonlyArray<string> | null {
  return current === null ? [] : null;
}

/**
 * Переключение одной опции группы в черновике шита: «все» (null) — явный
 * список «все, кроме переключённой» (полный каталог опций знает вызывающий
 * шит); список — добавить/убрать id. Обратное включение «всех» — не здесь:
 * мастер-чекбокс группы пишет null напрямую.
 */
export function toggleHistoryFilterOption<T extends string>(
  allOptions: ReadonlyArray<T>,
  selected: ReadonlyArray<T> | null,
  id: T,
): ReadonlyArray<T> | null {
  if (selected === null) {
    return allOptions.filter((option) => option !== id);
  }
  return selected.includes(id) ? selected.filter((option) => option !== id) : [...selected, id];
}
