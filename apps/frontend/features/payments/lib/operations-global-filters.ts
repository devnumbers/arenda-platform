import type { IsoDate } from '@/entities/payment';
import { pluralize } from '@/shared/lib/pluralize';
import {
  readOperationsFilters,
  type OperationsFilters,
  type OperationsParamsSource,
} from './operations-filters';

/**
 * Фильтры глобальной ленты «Операции» (#541): период и категории — те же
 * правила, что на объектных экранах (#477), плюс мультивыбор объектов
 * (решение владельца #539): состояние живёт в адресе
 * (?from=&to=&property=&category=), дефолт — текущий месяц, все объекты,
 * все категории.
 */
export type GlobalOperationsFilters = OperationsFilters & {
  /** Выбранные объекты (uuid, порядок выбора); пусто — «Все объекты». */
  readonly propertyIds: ReadonlyArray<string>;
};

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Чтение фильтров глобальной ленты: период и категории — правила
 * объектного экрана (битые даты/перевёрнутый период/будущий хвост
 * отбрасываются), объекты — непустые uuid без дублей, порядок выбора
 * сохраняется. */
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
  return { period, categories, propertyIds };
}

/** Параметры URL фильтров глобальной ленты: те же правила записи, что на
 * объектном экране (явный выбор пишется всегда, дефолт — пустой query),
 * объекты — comma-list по имени параметра `property`. */
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
  return result;
}

/** Лейбл чипа «Объект» (макет 1733-26805): без выбора — «Все объекты»,
 * один — «1 объект», несколько — счётчик со склонением. */
export function operationsPropertyChipLabel(propertyIds: ReadonlyArray<string>): string {
  if (propertyIds.length === 0) {
    return 'Все объекты';
  }
  return `${propertyIds.length} ${pluralize(propertyIds.length, 'объект', 'объекта', 'объектов')}`;
}
