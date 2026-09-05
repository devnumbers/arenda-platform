/**
 * Фильтр по объекту глобальной ленты «Задачи» (карта #518, тикет #524,
 * Figma 1726-88880/1726-86913): выбранная опция шита «Выбрать объект»
 * применяется в адрес страницы (?property=<uuid>) — ссылка шарится,
 * «назад» по истории возвращает к ленте без фильтра. Опции «Без объекта»
 * в фильтре нет (решение 7 #522): «Все объекты» — весь merged-фид вместе
 * с безобъектными, чип показывает имя выбранного объекта.
 */

/** Разобранный фильтр ленты: без параметра — «Все объекты». */
export type TasksFeedFilter = {
  readonly propertyId: string | null;
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

/** Чтение фильтра из URL: не-uuid и пустая строка отбрасываются — битая
 * ссылка открывает ленту без фильтра, а не с ошибкой. */
export function readTasksFeedFilter(params: TasksFeedFilterParamsSource): TasksFeedFilter {
  const raw = params.get(PROPERTY_PARAM);
  const propertyId = raw !== null && UUID_RE.test(raw) ? raw : null;
  return { propertyId };
}

/** Параметры адреса для фильтра — запись через useTasksFeedFilter. */
export function tasksFeedFilterParams(filter: TasksFeedFilter): Record<string, string> {
  if (filter.propertyId === null) {
    return {};
  }
  return { [PROPERTY_PARAM]: filter.propertyId };
}
