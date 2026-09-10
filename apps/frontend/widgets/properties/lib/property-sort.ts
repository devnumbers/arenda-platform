import type { Property } from '@/entities/property';
import { propertyTypeOptions } from '@/features/properties';

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

/** Основной объект выше любого не-основного; среди основных — первый
 * отмеченный (раньший pinned_at) выше, время повторной отметки не меняет. */
function comparePrimary(a: Property, b: Property): number {
  if (a.pinned_at && b.pinned_at) return a.pinned_at < b.pinned_at ? -1 : a.pinned_at > b.pinned_at ? 1 : 0;
  if (a.pinned_at) return -1;
  if (b.pinned_at) return 1;
  return 0;
}

export function sortProperties(
  items: readonly Property[],
  sort: PropertySort,
): Property[] {
  const direction = sort.direction === 'desc' ? -1 : 1;
  return [...items].sort((a, b) => {
    const byPrimary = comparePrimary(a, b);
    if (byPrimary !== 0) return byPrimary;
    return compareByField(a, b, sort.field) * direction;
  });
}

/** Клиентский поиск (#586): подстрока по названию и адресу без регистра. */
export function filterPropertiesByQuery(
  items: readonly Property[],
  query: string,
): Property[] {
  const needle = query.trim().toLowerCase();
  if (needle === '') return [...items];
  return items.filter(
    (property) =>
      property.name.toLowerCase().includes(needle)
      || property.address.toLowerCase().includes(needle),
  );
}

type SearchParamsLike = Record<string, string | string[] | undefined>;

function readString(value: string | string[] | undefined): string | undefined {
  if (Array.isArray(value)) {
    return value[0];
  }
  return value;
}

const sortFields = new Set<PropertySortField>([
  'name',
  'created',
  'type',
  'status',
]);

/** Разбор сортировки из URL (?sort=<поле>&order=<asc|desc>; легаси
 * name_asc/name_desc из старого списка читается, чтобы ссылки не протухли). */
export function parseSortFromParams(params: SearchParamsLike): PropertySort {
  const raw = readString(params.sort);

  if (raw === 'name_asc') return { field: 'name', direction: 'asc' };
  if (raw === 'name_desc') return { field: 'name', direction: 'desc' };

  if (raw !== undefined && sortFields.has(raw as PropertySortField)) {
    return {
      field: raw as PropertySortField,
      direction: readString(params.order) === 'desc' ? 'desc' : 'asc',
    };
  }

  return DEFAULT_PROPERTY_SORT;
}

/** Сериализация в URL: значения по умолчанию параметров не создают. */
export function serializeSortToParams(sort: PropertySort): Record<string, string> {
  const params: Record<string, string> = {};
  if (sort.field !== DEFAULT_PROPERTY_SORT.field) params.sort = sort.field;
  if (sort.direction !== DEFAULT_PROPERTY_SORT.direction) params.order = sort.direction;
  return params;
}
