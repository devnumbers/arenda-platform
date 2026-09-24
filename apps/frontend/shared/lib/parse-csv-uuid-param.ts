/**
 * Чтение CSV-группы из query-параметра адреса — канон мультивыбора
 * сущностей в адресе (объекты глобальной ленты операций #541, задачи #547,
 * участники/объекты фильтров истории #711, слаги действий/видов истории
 * #711 и категории операций #477): элементы чистятся по одному — пробелы
 * снимаются, пустые отбрасываются, дубли схлопываются, порядок первого
 * появления сохраняется; предикат валидности опционален — без него
 * остаются все непустые элементы. Ссылка с одним мусорным элементом
 * открывает срез остальных валидных, а не ломает экран.
 *
 * Семантику группы (null — «все», пустое значение — «ни один», список —
 * выбор) канон не различает: отсутствующий параметр и пустое значение
 * дают одинаковый пустой список — триаду держит вызывающий.
 */

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function readCsvParam(raw: string | null): string[];
export function readCsvParam<T extends string>(
  raw: string | null,
  isValid: (value: string) => value is T,
): T[];
export function readCsvParam(raw: string | null, isValid?: (value: string) => boolean): string[] {
  const result: string[] = [];
  const seen = new Set<string>();
  for (const part of (raw ?? '').split(',')) {
    const value = part.trim();
    if (value.length === 0 || seen.has(value) || (isValid !== undefined && !isValid(value))) {
      continue;
    }
    seen.add(value);
    result.push(value);
  }
  return result;
}

/** uuid-вариант канона: из группы остаются только валидные uuid. */
export function readCsvUuidParam(raw: string | null): string[] {
  return readCsvParam(raw, (value): value is string => UUID_RE.test(value));
}
