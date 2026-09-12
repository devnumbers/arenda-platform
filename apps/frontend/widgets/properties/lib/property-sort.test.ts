import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import {
  DEFAULT_PROPERTY_SORT,
  parseSortFromParams,
  serializeSortToParams,
  sortProperties,
} from './property-sort';

function makeProperty(overrides: Partial<Property> & { id: string }): Property {
  return {
    name: 'Квартира',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    pinned_at: null,
    ...overrides,
  };
}

const A = makeProperty({ id: 'a', name: 'Анапа', created_at: '2025-01-01T00:00:00Z' });
const B = makeProperty({ id: 'b', name: 'Борис', created_at: '2026-02-01T00:00:00Z' });
const V = makeProperty({ id: 'v', name: 'Волга', created_at: '2026-01-15T00:00:00Z' });

describe('sortProperties: по названию', () => {
  it('возрастание — алфавит А-Я', () => {
    expect(sortProperties([V, A, B], { field: 'name', direction: 'asc' }).map((p) => p.id)).toEqual(['a', 'b', 'v']);
  });

  it('убывание — алфавит Я-А', () => {
    expect(sortProperties([V, A, B], { field: 'name', direction: 'desc' }).map((p) => p.id)).toEqual(['v', 'b', 'a']);
  });
});

describe('sortProperties: по дате создания', () => {
  it('возрастание — старые сверху', () => {
    expect(sortProperties([B, A, V], { field: 'created', direction: 'asc' }).map((p) => p.id)).toEqual(['a', 'v', 'b']);
  });

  it('убывание — новые сверху', () => {
    expect(sortProperties([B, A, V], { field: 'created', direction: 'desc' }).map((p) => p.id)).toEqual(['b', 'v', 'a']);
  });
});

describe('sortProperties: по типу объекта', () => {
  // Канон визарда: квартира → комната → апартаменты → дом → …
  const apartment = makeProperty({ id: 'apt', name: 'Ялта', type: 'apartment' });
  const room = makeProperty({ id: 'room', name: 'Админ', type: 'room' });
  const house = makeProperty({ id: 'house', name: 'Борки', type: 'house' });

  it('возрастание — порядок типов канона визарда', () => {
    expect(sortProperties([house, apartment, room], { field: 'type', direction: 'asc' }).map((p) => p.id)).toEqual([
      'apt',
      'room',
      'house',
    ]);
  });

  it('убывание — обратный порядок типов', () => {
    expect(sortProperties([house, apartment, room], { field: 'type', direction: 'desc' }).map((p) => p.id)).toEqual([
      'house',
      'room',
      'apt',
    ]);
  });

  it('внутри одного типа — по названию', () => {
    const aptB = makeProperty({ id: 'apt-b', name: 'Анапа', type: 'apartment' });
    expect(sortProperties([apartment, aptB], { field: 'type', direction: 'asc' }).map((p) => p.id)).toEqual([
      'apt-b',
      'apt',
    ]);
  });
});

describe('sortProperties: по статусу (резолюция #584)', () => {
  const needsAttention = makeProperty({
    id: 'na',
    name: 'Аляска',
    occupancy: { status: 'needs_attention', start_date: '2025-01-01', planned_end_date: '2026-01-01' },
  });
  const active = makeProperty({
    id: 'act',
    name: 'Байкал',
    occupancy: { status: 'active', start_date: '2025-01-01', planned_end_date: '2027-01-01' },
  });
  const upcoming = makeProperty({
    id: 'up',
    name: 'Вега',
    occupancy: { status: 'upcoming', start_date: '2027-01-01', planned_end_date: null },
  });
  const maintenanceNoRental = makeProperty({ id: 'mnt', name: 'Гараж', status: 'maintenance' });
  const none = makeProperty({ id: 'none', name: 'Дача' });
  // Объект с арендой на ремонте живёт в своей арендной группе.
  const maintenanceWithRental = makeProperty({
    id: 'mnt-rent',
    name: 'Ёлка',
    status: 'maintenance',
    occupancy: { status: 'active', start_date: '2025-01-01', planned_end_date: null },
  });

  it('возрастание: подошла к концу → с арендой (вкл. на ремонте) → на ремонте → без аренды', () => {
    const sorted = sortProperties(
      [none, maintenanceNoRental, upcoming, active, maintenanceWithRental, needsAttention],
      { field: 'status', direction: 'asc' },
    );
    expect(sorted.map((p) => p.id)).toEqual(['na', 'act', 'up', 'mnt-rent', 'mnt', 'none']);
  });

  it('убывание — обратный порядок групп', () => {
    const sorted = sortProperties([needsAttention, none], { field: 'status', direction: 'desc' });
    expect(sorted.map((p) => p.id)).toEqual(['none', 'na']);
  });
});

describe('sortProperties: основной объект всегда первый', () => {
  const pinnedFirst = makeProperty({ id: 'p1', name: 'Ялта', pinned_at: '2026-09-01T00:00:00Z' });
  const pinnedSecond = makeProperty({ id: 'p2', name: 'Анапа', pinned_at: '2026-09-05T00:00:00Z' });

  it('основные выше остальных при любом поле', () => {
    expect(sortProperties([A, pinnedFirst, B], { field: 'name', direction: 'asc' }).map((p) => p.id)).toEqual([
      'p1',
      'a',
      'b',
    ]);
  });

  it('среди основных — первый отмеченный (раньший pinned_at) выше', () => {
    expect(sortProperties([pinnedSecond, pinnedFirst], DEFAULT_PROPERTY_SORT).map((p) => p.id)).toEqual(['p1', 'p2']);
  });

  it('основной первый даже при сортировке по статусу', () => {
    const nonePinned = makeProperty({ id: 'p3', name: 'Дача', pinned_at: '2026-09-02T00:00:00Z' });
    const needsAttention = makeProperty({
      id: 'na',
      name: 'Аляска',
      occupancy: { status: 'needs_attention', start_date: '2025-01-01', planned_end_date: '2026-01-01' },
    });
    const sorted = sortProperties([needsAttention, nonePinned], { field: 'status', direction: 'asc' });
    expect(sorted.map((p) => p.id)).toEqual(['p3', 'na']);
  });
});

describe('parse/serialize сортировки в URL', () => {
  it('по умолчанию — название по возрастанию', () => {
    expect(parseSortFromParams({})).toEqual({ field: 'name', direction: 'asc' });
  });

  it('читает новые параметры sort/order', () => {
    expect(parseSortFromParams({ sort: 'created', order: 'desc' })).toEqual({ field: 'created', direction: 'desc' });
    expect(parseSortFromParams({ sort: 'status' })).toEqual({ field: 'status', direction: 'asc' });
  });

  it('читает легаси name_asc/name_desc', () => {
    expect(parseSortFromParams({ sort: 'name_asc' })).toEqual({ field: 'name', direction: 'asc' });
    expect(parseSortFromParams({ sort: 'name_desc' })).toEqual({ field: 'name', direction: 'desc' });
  });

  it('мусор в параметрах — значение по умолчанию', () => {
    expect(parseSortFromParams({ sort: 'цвет' })).toEqual(DEFAULT_PROPERTY_SORT);
    expect(parseSortFromParams({ sort: 'name', order: 'вверх' })).toEqual({ field: 'name', direction: 'asc' });
  });

  it('сериализация опускает значения по умолчанию', () => {
    expect(serializeSortToParams(DEFAULT_PROPERTY_SORT)).toEqual({});
    expect(serializeSortToParams({ field: 'type', direction: 'asc' })).toEqual({ sort: 'type' });
    expect(serializeSortToParams({ field: 'created', direction: 'desc' })).toEqual({ sort: 'created', order: 'desc' });
  });
});
