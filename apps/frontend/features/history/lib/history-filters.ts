import type { IsoDate, IsoRange } from '@/shared/lib/calendar';
import type { HistoryBaseAction, HistoryKind, HistoryParticipantOption } from '@/entities/history';
import { HISTORY_BASE_ACTIONS, HISTORY_KINDS } from '@/entities/history';
import { formatIsoRangeChipLabel } from '@/shared/lib/date-format';
import { displayOwnUserName } from '@/shared/lib/display-name';
import { readCsvParam, readCsvUuidParam } from '@/shared/lib/parse-csv-uuid-param';
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

const KIND_SET = new Set<string>(HISTORY_KINDS);

/** CSV-группа uuid с семантикой группы шита: параметр отсутствует — «все»
 * (null), значение чистит канон readCsvUuidParam. */
function readUuidGroup(raw: string | null): ReadonlyArray<string> | null {
  return raw === null ? null : readCsvUuidParam(raw);
}

/** CSV-группа слагов с семантикой группы шита: параметр отсутствует —
 * «все» (null), значение чистит канон readCsvParam (валидные слаги, без
 * дублей, порядок сохранён). */
function readCsvGroup<T extends string>(
  raw: string | null,
  isValid: (value: string) => value is T,
): ReadonlyArray<T> | null {
  return raw === null ? null : readCsvParam(raw, isValid);
}

/** Чтение фильтров из адреса: период — канон readIsoRangeParam (правила
 * #477: битые даты, перевёрнутый период отбрасываются; будущий хвост
 * обрезается «сегодня»), группы — чистка валидных значений без дублей
 * (слаги и uuid — каноны readCsvParam/readCsvUuidParam). */
export function readHistoryFilters(params: HistoryParamsSource, today: IsoDate): HistoryFilters {
  return {
    period: readIsoRangeParam(params, 'from', 'to', today),
    actions: readCsvGroup(params.get('actions'), (value): value is HistoryBaseAction =>
      HISTORY_BASE_ACTIONS.includes(value as HistoryBaseAction),
    ),
    kinds: readCsvGroup(params.get('kinds'), (value): value is HistoryKind => KIND_SET.has(value)),
    actorIds: readUuidGroup(params.get('actors')),
    propertyIds: readUuidGroup(params.get('objects')),
  };
}

/**
 * Patch адреса (канон: дефолтные значения не пишутся): «все» (null) — ключ
 * отсутствует, «ни один» — пустое значение (отличимо от отсутствия), выбор
 * — comma-list. from/to пишутся парой всегда, когда период применён.
 * Период матчится сервером как UTC-сутки (канон админ-аудита), тогда как
 * группировка дней ленты — по локальному дню смотрящего; расхождение
 * краевых часов принято (#453, ADR 0061 §7).
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
 * Пины страницы: предметы, прибитые путём страницы, а не фильтрами
 * («История объекта» #840 — объект, «Действия участника в объекте»
 * #841 — пара, страница участника #1582 — человек со скоупом его ног,
 * аменда владельца 22.10.2026). Заданный пин значит «своя группа адреса
 * не читается»: в шите она видна серой незабираемой строкой (макеты
 * 2184-92510, 2184-94176; на странице пары — обе, «1/1»), но не пишет в
 * черновик; hand-crafted ?actors=/?objects= игнорируются, а «Применить
 * фильтры» их вычищает — запись группы своя.
 */
export type HistoryFilterPins = {
  /** Прибитый человек (#841 и страница участника #1582): uuid юзера или
   * email pending-агрегата (#693) из пути. */
  readonly actorId?: string;
  /** Прибитый объект (#840): uuid объекта из пути. */
  readonly propertyId?: string;
};

/**
 * Фильтры адреса с занулением прибитых групп: пин задан — своя группа
 * не читается (HistoryFilterPins), неприбитые группы проходят без
 * изменений; без пинов фильтры как есть (общая лента #711).
 */
export function pinnedHistoryFilters(
  filters: HistoryFilters,
  pins?: HistoryFilterPins,
): HistoryFilters {
  if (pins === undefined) {
    return filters;
  }
  return {
    ...filters,
    actorIds: pins.actorId !== undefined ? null : filters.actorIds,
    propertyIds: pins.propertyId !== undefined ? null : filters.propertyIds,
  };
}

/**
 * Скоуп ленты GET /history (#708) из фильтров; null — какая-то из групп
 * выбрана «в ноль»: по семантике фильтра результат пуст, а серверный
 * контракт пустой список не отличает от «все» — лента не запрашивается
 * вовсе (экран рисует пустое состояние без запроса).
 *
 * С пинами (ADR 0061 §7 — на прибитых страницах #840/#841 тот же
 * GET /history, actor_ids и property_ids = по одному; сервер AND'ит их —
 * бэк #708 без изменений) группы адреса действуют поверх прибитых
 * предметов, а прибитые id подставляются в скоуп сильнее своих групп:
 * даже «ни одного» ([]) прибитая группа ленту не опустошает.
 */
export function historyFeedScope(
  filters: HistoryFilters,
  pins?: HistoryFilterPins,
): HistoryFeedScope | null {
  const pinned = pinnedHistoryFilters(filters, pins);
  if (
    (pinned.actions !== null && pinned.actions.length === 0) ||
    (pinned.kinds !== null && pinned.kinds.length === 0) ||
    (pinned.actorIds !== null && pinned.actorIds.length === 0) ||
    (pinned.propertyIds !== null && pinned.propertyIds.length === 0)
  ) {
    return null;
  }
  return {
    dateFrom: pinned.period?.from,
    dateTo: pinned.period?.to,
    actions: pinned.actions ?? undefined,
    kinds: pinned.kinds ?? undefined,
    actorIds: pins?.actorId !== undefined ? [pins.actorId] : (pinned.actorIds ?? undefined),
    propertyIds:
      pins?.propertyId !== undefined ? [pins.propertyId] : (pinned.propertyIds ?? undefined),
  };
}

/** Лейбл чипа периода шита: без периода — «Выбрать период» (макет
 * 2177-60527), с периодом — формат чипа канона (макет 2067-162950). */
export function historyPeriodChipLabel(period: IsoRange | null): string {
  return period !== null ? formatIsoRangeChipLabel(period) : 'Выбрать период';
}

/** Заголовок строки участника шита: чужой — канон отображаемого имени,
 * себе — база своей строки displayOwnUserName (макет 2067-163528:
 * «Даниил (Вы)»; серый суффикс рисует шит через MeSuffix, #1356).
 * Фамилия своей строки в канон D4 не подмешивается, и без имени канон
 * даёт «Пользователь» (#1105, аменд #1123). */
export function historyParticipantTitle(
  participant: Pick<HistoryParticipantOption, 'name' | 'firstName'>,
  isMe: boolean,
): string {
  if (!isMe) {
    return participant.name;
  }
  return displayOwnUserName(participant.firstName);
}

/** Секция группы «Участники» шита: подпись (undefined — секция без
 * подписи, разделение не показывается) и строки секции. */
export type HistoryParticipantSection = {
  readonly title?: string;
  readonly participants: ReadonlyArray<HistoryParticipantOption>;
};

/** Раздел «Текущие» группы «Участники» (D6, карта #1324; макет
 * 3339-160075): живые ноги скоупа — владелец и членство (включая
 * приостановленное). */
const HISTORY_CURRENT_SECTION_TITLE = 'Текущие';

/** Раздел «Удалённые» (D6): только строки журнала — ревок или выход;
 * подпись живёт только при настоящем разделении (см.
 * historyParticipantSections). */
const HISTORY_REMOVED_SECTION_TITLE = 'Удалённые';

/**
 * Секции группы «Участники» по статусу D6 (current | removed с
 * /history/filters, бекенд-слайс #1318). Подписанное разделение
 * показывается, только когда есть и текущие, и удалённые (parent #1294:
 * «после того, как участника удалили из объекта, если он что либо делал…»)
 * — без удалённых и в оборонительном случае «только удалённые» (прибитая
 * строка отозванного #712) одна секция без подписи. Порядок внутри секций
 * — порядок опций сервера (алфавит SQL).
 */
export function historyParticipantSections(
  participants: ReadonlyArray<HistoryParticipantOption>,
): ReadonlyArray<HistoryParticipantSection> {
  const removed = participants.filter((participant) => participant.status === 'removed');
  if (removed.length === 0 || removed.length === participants.length) {
    return participants.length === 0 ? [] : [{ participants }];
  }
  return [
    {
      title: HISTORY_CURRENT_SECTION_TITLE,
      participants: participants.filter((participant) => participant.status !== 'removed'),
    },
    { title: HISTORY_REMOVED_SECTION_TITLE, participants: removed },
  ];
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
