/**
 * Фильтр по объектам глобальной ленты «Задачи» (карта #518, тикеты #524 и
 * #547, Figma 1726-88880/1726-86913): выбранные опции шита «Выбрать объект»
 * применяются в адрес страницы (?property=<uuid>[,<uuid>…]) — ссылка
 * шарится, «назад» по истории возвращает к ленте без фильтра. Мультивыбор —
 * накопление чекбоксов (решение владельца 2026-09-05), «Все объекты» —
 * пустой список, весь merged-фид вместе с безобъектными. Опции «Без
 * объекта» в фильтре нет (решение 7 #522).
 */

/** Разобранный фильтр ленты: пустой список — «Все объекты». */
export type TasksFeedFilter = {
  readonly propertyIds: ReadonlyArray<string>;
};

/** Минимальный источник параметров — ReadonlyURLSearchParams Next ему
 * удовлетворяет; структурный тип держит модуль чистым для vitest. */
export type TasksFeedFilterParamsSource = {
  readonly get: (name: string) => string | null;
};

const PROPERTY_PARAM = 'property';
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Имя параметра адреса — единый источник для чтения и записи. */
export const tasksFeedPropertyParam = PROPERTY_PARAM;

/** Чтение фильтра из URL: битые элементы отбрасываются по одному, дубли
 * схлопываются — ссылка с одним мусорным id открывает срез остальных, а не
 * весь фид; одни битые — ленту без фильтра. */
export function readTasksFeedFilter(params: TasksFeedFilterParamsSource): TasksFeedFilter {
  const raw = params.get(PROPERTY_PARAM);
  if (raw === null) {
    return { propertyIds: [] };
  }
  const seen = new Set<string>();
  for (const part of raw.split(',')) {
    const id = part.trim();
    if (UUID_RE.test(id)) {
      seen.add(id);
    }
  }
  return { propertyIds: [...seen] };
}

/** Параметры адреса для фильтра — запись через useTasksFeedFilter. */
export function tasksFeedFilterParams(filter: TasksFeedFilter): Record<string, string> {
  if (filter.propertyIds.length === 0) {
    return {};
  }
  return { [PROPERTY_PARAM]: filter.propertyIds.join(',') };
}
