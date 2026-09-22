import type { IsoDate } from '@/entities/payment';
import { buildUrlWithParams } from '@/shared/lib/url-params';
import { pluralize } from '@/shared/lib/pluralize';
import { safeInternalPath } from '@/shared/lib/safe-internal-path';
import {
  readOperationsFilters,
  type OperationsFilters,
  type OperationsParamsSource,
} from './operations-filters';

/**
 * Фильтры глобальной ленты «Операции» (#541): период и категории — те же
 * правила, что на объектных экранах (#477), плюс мультивыбор объектов
 * (решение владельца #539) и опция архива (#549): состояние живёт в адресе
 * (?from=&to=&property=&category=&archived=), дефолт — весь период
 * (#670, карта #669), все объекты, все категории, архив исключён.
 */
export type GlobalOperationsFilters = OperationsFilters & {
  /** Выбранные объекты (uuid, порядок выбора); пусто — «Все объекты». */
  readonly propertyIds: ReadonlyArray<string>;
  /** Операции архивных объектов включены в скоуп (#549, ?archived=1). */
  readonly archived: boolean;
};

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Куда возвращаться со страниц выбора фильтров глобальной ленты (#542):
 * только маршруты зоны /operations (главная лента, поиск #543, категории
 * #544 открывают друг друга с ?return=), иначе — главный список. Тот же
 * приём, что resolveFilterReturnPath у объектных экранов (#477).
 */
export function resolveGlobalFilterReturnPath(raw: string | null): string {
  const candidate = safeInternalPath(raw);
  const path = candidate?.split('?')[0] ?? '';
  if (path === '/operations' || path.startsWith('/operations/')) {
    return candidate as string;
  }
  return '/operations';
}

/** Чтение фильтров глобальной ленты: период и категории — правила
 * объектного экрана (битые даты/перевёрнутый период/будущий хвост
 * отбрасываются), объекты — непустые uuid без дублей, порядок выбора
 * сохраняется; архив включён только явным archived=1 (#549). */
export function readGlobalOperationsFilters(
  params: OperationsParamsSource,
  today: IsoDate,
): GlobalOperationsFilters {
  const { period, categories } = readOperationsFilters(params, today);
  const propertyIds: string[] = [];
  const seen = new Set<string>();
  for (const raw of (params.get('property') ?? '').split(',')) {
    const candidate = raw.trim();
    if (candidate.length === 0 || seen.has(candidate) || !UUID_RE.test(candidate)) {
      continue;
    }
    seen.add(candidate);
    propertyIds.push(candidate);
  }
  return { period, categories, propertyIds, archived: params.get('archived') === '1' };
}

/** Параметры URL фильтров глобальной ленты: те же правила записи, что на
 * объектном экране (явный выбор пишется всегда, дефолт — пустой query),
 * объекты — comma-list по имени параметра `property`, архив — archived=1
 * только когда включён (#549). */
export function globalOperationsFiltersParams(
  filters: GlobalOperationsFilters,
): Record<string, string> {
  const result: Record<string, string> = {};
  if (filters.period !== null) {
    result.from = filters.period.from;
    result.to = filters.period.to;
  }
  if (filters.categories.length > 0) {
    result.category = filters.categories.join(',');
  }
  if (filters.propertyIds.length > 0) {
    result.property = filters.propertyIds.join(',');
  }
  if (filters.archived) {
    result.archived = '1';
  }
  return result;
}

/**
 * Ссылка на экран глобальной ленты с текущими фильтрами, extra — поверх
 * (например, ?return= страниц выбора). Дефолтные фильтры дают чистую базу
 * без query — канон buildUrlWithParams (#792, «пустой query — голый
 * адрес»), которому функция делегирует; одна функция вместо двух дословных
 * filterHref ленты и направления (хвост #792, находка повторного
 * pre-merge #785).
 */
export function globalOperationsFiltersHref(
  base: string,
  filters: GlobalOperationsFilters,
  extra?: Record<string, string>,
): string {
  const params = new URLSearchParams(globalOperationsFiltersParams(filters));
  for (const [name, value] of Object.entries(extra ?? {})) {
    params.set(name, value);
  }
  return buildUrlWithParams(base, params);
}

/** Лейбл чипа «Объект» (макет 1733-26805): без выбора — «Все объекты»,
 * один — «1 объект», несколько — счётчик со склонением. */
export function operationsPropertyChipLabel(propertyIds: ReadonlyArray<string>): string {
  if (propertyIds.length === 0) {
    return 'Все объекты';
  }
  return `${propertyIds.length} ${pluralize(propertyIds.length, 'объект', 'объекта', 'объектов')}`;
}
