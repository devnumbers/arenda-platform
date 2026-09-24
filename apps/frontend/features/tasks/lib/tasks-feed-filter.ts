/**
 * Фильтр глобальной ленты «Задачи» (карта #518, тикеты #524/#547, Figma
 * 1726-88880): выбранные опции страницы «Выбрать объект» применяются в адрес
 * страницы (?property=<uuid>[,<uuid>…] и ?withoutProperty=1) — ссылка
 * шарится, «назад» по истории возвращает к ленте без фильтра. Мультивыбор —
 * накопление чекбоксов (решение владельца 2026-09-05); «Общие задачи» —
 * отдельный чекбокс, свободно совмещается с объектами — union-фид (решение
 * владельца 2026-09-07); «Все задачи» — оба пустые, весь merged-фид.
 */

import { readCsvUuidParam } from '@/shared/lib/parse-csv-uuid-param';

/** Разобранный фильтр ленты: пустые оба поля — «Все задачи». */
export type TasksFeedFilter = {
  readonly propertyIds: ReadonlyArray<string>;
  readonly withoutProperty: boolean;
};

/** Пустой фильтр — «Все задачи», весь merged-фид. */
export const EMPTY_TASKS_FEED_FILTER: TasksFeedFilter = {
  propertyIds: [],
  withoutProperty: false,
};

/** Минимальный источник параметров — ReadonlyURLSearchParams Next ему
 * удовлетворяет; структурный тип держит модуль чистым для vitest. */
export type TasksFeedFilterParamsSource = {
  readonly get: (name: string) => string | null;
};

const PROPERTY_PARAM = 'property';
const WITHOUT_PROPERTY_PARAM = 'withoutProperty';

/** Имена параметров адреса — единый источник для чтения и записи. */
export const tasksFeedPropertyParam = PROPERTY_PARAM;
export const tasksFeedWithoutPropertyParam = WITHOUT_PROPERTY_PARAM;

/** Чтение фильтра из URL: объекты — канон readCsvUuidParam (битые элементы
 * отбрасываются по одному, дубли схлопываются — ссылка с одним мусорным id
 * открывает срез остальных, а не весь фид; одни битые — ленту без
 * фильтра). withoutProperty читают только «1»/«true», прочие значения
 * молча выключают флаг. */
export function readTasksFeedFilter(params: TasksFeedFilterParamsSource): TasksFeedFilter {
  const withoutRaw = params.get(WITHOUT_PROPERTY_PARAM);
  return {
    propertyIds: readCsvUuidParam(params.get(PROPERTY_PARAM)),
    withoutProperty: withoutRaw === '1' || withoutRaw === 'true',
  };
}

/** Параметры адреса для фильтра — запись через useTasksFeedFilter. */
export function tasksFeedFilterParams(filter: TasksFeedFilter): Record<string, string> {
  const params: Record<string, string> = {};
  if (filter.propertyIds.length > 0) {
    params[PROPERTY_PARAM] = filter.propertyIds.join(',');
  }
  if (filter.withoutProperty) {
    params[WITHOUT_PROPERTY_PARAM] = '1';
  }
  return params;
}
