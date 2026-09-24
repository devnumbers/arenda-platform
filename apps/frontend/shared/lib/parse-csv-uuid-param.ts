/**
 * Чтение CSV-группы uuid из query-параметра адреса — канон мультивыбора
 * сущностей в адресе (объекты глобальной ленты операций #541, задачи #547,
 * участники/объекты фильтров истории #711): элементы чистятся по одному —
 * пробелы снимаются, пустые и не-uuid отбрасываются, дубли схлопываются,
 * порядок первого появления сохраняется; отсутствующий параметр — пустой
 * список. Ссылка с одним мусорным id открывает срез остальных валидных, а
 * не ломает ленту.
 */

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function readCsvUuidParam(raw: string | null): string[] {
  const result: string[] = [];
  const seen = new Set<string>();
  for (const part of (raw ?? '').split(',')) {
    const value = part.trim();
    if (value.length === 0 || seen.has(value) || !UUID_RE.test(value)) {
      continue;
    }
    seen.add(value);
    result.push(value);
  }
  return result;
}
