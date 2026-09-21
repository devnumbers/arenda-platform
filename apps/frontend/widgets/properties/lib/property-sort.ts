import type { Property } from '@/entities/property';
import { comparePrimaryProperty } from '@/entities/property';
import { propertyTypeOptions } from '@/features/properties';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/**
 * Сортировка списка объектов (тикет #586, резолюция #584): поле
 * (название / дата создания / тип объекта / статус) и направление
 * (возрастание / убывание) — два независимых «радио» PickerMenu.
 * Основной объект всегда первый (канон «Основной объект», #584): при любом
 * поле и направлении основные идут выше остальных, среди основных —
 * первый отмеченный (раньший pinned_at) выше.
 */
export type PropertySortField = 'name' | 'created' | 'type' | 'status';
export type PropertySortDirection = 'asc' | 'desc';

export type PropertySort = {
  readonly field: PropertySortField;
  readonly direction: PropertySortDirection;
};

export const DEFAULT_PROPERTY_SORT: PropertySort = {
  field: 'name',
  direction: 'asc',
};

/** Короткая подпись чипа сортировки (в пикере — «По …»). */
export const SORT_FIELD_CHIP_LABEL: Record<PropertySortField, string> = {
  name: 'Название',
  created: 'Дата создания',
  type: 'Тип объекта',
  status: 'Статус',
};

/** Опции поля сортировки пикера, порядок как в макете (1603:93153). */
export const SORT_FIELD_OPTIONS: ReadonlyArray<PropertySortField> = [
  'name',
  'created',
  'type',
  'status',
];

/** Группа занятости для сортировки «По статусу» (резолюция #584):
 * подошла к концу → с арендой (активные и будущие вперемешку) →
 * на ремонте (только безарендные) → без аренды. */
function statusGroup(property: Property): number {
  const occupancy = property.occupancy?.status;
  if (occupancy === 'needs_attention') return 0;
  if (occupancy === 'active' || occupancy === 'upcoming') return 1;
  if (property.status === 'maintenance') return 2;
  return 3;
}

const typeOrder = new Map(
  propertyTypeOptions.map((option, index) => [option.value, index]),
);

/** Поле-сравнение в возрастательном направлении; направление умножением. */
function compareByField(a: Property, b: Property, field: PropertySortField): number {
  switch (field) {
    case 'name':
      return a.name.localeCompare(b.name, 'ru');
    case 'created':
      return Date.parse(a.created_at) - Date.parse(b.created_at);
    case 'type': {
      const byType = (typeOrder.get(a.type) ?? 0) - (typeOrder.get(b.type) ?? 0);
      return byType !== 0 ? byType : a.name.localeCompare(b.name, 'ru');
    }
    case 'status': {
      const byGroup = statusGroup(a) - statusGroup(b);
      return byGroup !== 0 ? byGroup : a.name.localeCompare(b.name, 'ru');
    }
  }
}

export function sortProperties(
  items: readonly Property[],
  sort: PropertySort,
): Property[] {
  const direction = sort.direction === 'desc' ? -1 : 1;
  return [...items].sort((a, b) => {
    // Основной объект выше при любом поле/направлении — правило
    // pinned_at живёт в каноне entities/property (#584).
    const byPrimary = comparePrimaryProperty(a, b);
    if (byPrimary !== 0) return byPrimary;
    return compareByField(a, b, sort.field) * direction;
  });
}

type SearchParamsLike = Record<string, string | string[] | undefined>;

/** Легаси-значение ?sort=name_asc|name_desc из старого списка: ссылки не
 * протухают. Читается только строка — массив по канону parseEnumParam бит. */
function parseLegacySortParam(value: string | string[] | undefined): PropertySort | null {
  if (value === 'name_asc') return DEFAULT_PROPERTY_SORT;
  if (value === 'name_desc') return { field: 'name', direction: 'desc' };
  return null;
}

/** Разбор сортировки из URL (?sort=<поле>&order=<asc|desc>): политика
 * канона parseEnumParam — отсутствующее, пустое, неизвестное и массивное
 * (битый дубликат) — дефолт; легаси name_asc/name_desc читается, чтобы
 * ссылки не протухли. */
export function parseSortFromParams(params: SearchParamsLike): PropertySort {
  const legacy = parseLegacySortParam(params.sort);
  if (legacy !== null) {
    return legacy;
  }
  return {
    field: parseEnumParam(params.sort, SORT_FIELD_OPTIONS, DEFAULT_PROPERTY_SORT.field),
    direction: parseEnumParam(params.order, ['asc', 'desc'], DEFAULT_PROPERTY_SORT.direction),
  };
}

/** Собственные параметры сортировки в адресе — знание этого модуля;
 * писатель (PropertiesPage) импортирует отсюда. */
export const PROPERTY_SORT_PARAMS = ['sort', 'order'] as const;

/** Сериализация в URL: значения по умолчанию параметров не создают. */
export function serializeSortToParams(sort: PropertySort): Record<string, string> {
  const params: Record<string, string> = {};
  if (sort.field !== DEFAULT_PROPERTY_SORT.field) params.sort = sort.field;
  if (sort.direction !== DEFAULT_PROPERTY_SORT.direction) params.order = sort.direction;
  return params;
}
