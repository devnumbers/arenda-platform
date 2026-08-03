// Самодостаточная копия логики форматирования характеристик объекта.
// Зеркалирует apps/frontend/features/property-attributes (catalog, labels, format)
// и должна оставаться синхронизированной с ним и с apps/backend/api/openapi/openapi.yaml.
// Read-only блок для show-вью админки (issue #133).

export type PropertyType =
  | 'apartment'
  | 'room'
  | 'apartments'
  | 'house'
  | 'commercial'
  | 'office'
  | 'warehouse'
  | 'garage'
  | 'parking'
  | 'land';

export type PropertyAttributes = Readonly<Record<string, string | number>>;

export type AttrKey =
  | 'rooms'
  | 'area_total'
  | 'area_living'
  | 'area_kitchen'
  | 'floor'
  | 'floors_total'
  | 'bathroom'
  | 'balcony'
  | 'renovation'
  | 'year_built'
  | 'ceiling_height'
  | 'parking_type'
  | 'land_area'
  | 'land_type'
  | 'house_type'
  | 'material'
  | 'shower'
  | 'building_type'
  | 'entrance'
  | 'parking_location'
  | 'parking_level'
  | 'spot_number';

export type AttrGroup = 'about_object' | 'about_building' | 'about_house' | 'about_land';

export type AttrField =
  | { readonly key: AttrKey; readonly kind: 'enum'; readonly options: readonly string[]; readonly group: AttrGroup | null }
  | { readonly key: AttrKey; readonly kind: 'number'; readonly unit: 'м²' | 'сотки' | 'м'; readonly min: number; readonly max: number; readonly decimals: number; readonly group: AttrGroup | null }
  | { readonly key: AttrKey; readonly kind: 'integer'; readonly min: number; readonly max: number; readonly group: AttrGroup | null }
  | { readonly key: AttrKey; readonly kind: 'string'; readonly maxLen: number; readonly group: AttrGroup | null };

export type AttrFieldList = readonly AttrField[];

const yearBuiltMax = new Date().getFullYear() + 5;

const apartmentFields: AttrFieldList = [
  { key: 'rooms', kind: 'enum', options: ['studio', '1', '2', '3', '4', '5', '6', '7_plus'], group: 'about_object' },
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: 'about_object' },
  { key: 'area_living', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: 'about_object' },
  { key: 'area_kitchen', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: 'about_object' },
  { key: 'floor', kind: 'integer', min: -3, max: 200, group: 'about_object' },
  { key: 'floors_total', kind: 'integer', min: 1, max: 200, group: 'about_object' },
  { key: 'bathroom', kind: 'enum', options: ['combined', 'separate', 'multiple'], group: 'about_object' },
  { key: 'balcony', kind: 'enum', options: ['none', 'balcony', 'loggia', 'balcony_and_loggia'], group: 'about_object' },
  { key: 'renovation', kind: 'enum', options: ['cosmetic', 'euro', 'design', 'required'], group: 'about_object' },
  { key: 'year_built', kind: 'integer', min: 1800, max: yearBuiltMax, group: 'about_building' },
  { key: 'ceiling_height', kind: 'number', unit: 'м', min: 2, max: 10, decimals: 2, group: 'about_building' },
  { key: 'parking_type', kind: 'enum', options: ['closed', 'underground', 'open'], group: 'about_building' },
];

const roomFields: AttrFieldList = [
  { key: 'rooms', kind: 'enum', options: ['2', '3', '4', '5', '6', '7_plus'], group: 'about_object' },
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: 'about_object' },
  { key: 'area_kitchen', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: 'about_object' },
  { key: 'floor', kind: 'integer', min: -3, max: 200, group: 'about_object' },
  { key: 'floors_total', kind: 'integer', min: 1, max: 200, group: 'about_object' },
  { key: 'bathroom', kind: 'enum', options: ['combined', 'separate', 'multiple'], group: 'about_object' },
  { key: 'balcony', kind: 'enum', options: ['none', 'balcony', 'loggia', 'balcony_and_loggia'], group: 'about_object' },
  { key: 'renovation', kind: 'enum', options: ['cosmetic', 'euro', 'design', 'required'], group: 'about_object' },
  { key: 'year_built', kind: 'integer', min: 1800, max: yearBuiltMax, group: 'about_building' },
];

const houseFields: AttrFieldList = [
  { key: 'land_area', kind: 'number', unit: 'сотки', min: 0.01, max: 1000000, decimals: 2, group: 'about_land' },
  { key: 'land_type', kind: 'enum', options: ['izhs', 'garden', 'farm'], group: 'about_land' },
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: 'about_house' },
  { key: 'floors_total', kind: 'integer', min: 1, max: 200, group: 'about_house' },
  { key: 'rooms', kind: 'enum', options: ['1', '2', '3', '4', '5', '6', '7_plus'], group: 'about_house' },
  { key: 'house_type', kind: 'enum', options: ['detached', 'part', 'townhouse', 'duplex'], group: 'about_house' },
  {
    key: 'material',
    kind: 'enum',
    options: ['brick', 'monolithic', 'panel', 'brick_monolithic', 'block', 'wooden', 'reinforced_concrete'],
    group: 'about_house',
  },
  { key: 'bathroom', kind: 'enum', options: ['indoor', 'outdoor', 'none'], group: 'about_house' },
  { key: 'shower', kind: 'enum', options: ['indoor', 'outdoor', 'none'], group: 'about_house' },
  { key: 'year_built', kind: 'integer', min: 1800, max: yearBuiltMax, group: 'about_house' },
];

const officeFields: AttrFieldList = [
  {
    key: 'building_type',
    kind: 'enum',
    options: ['business_center', 'warehouse_building', 'shopping_center', 'detached', 'residential'],
    group: null,
  },
  { key: 'floor', kind: 'integer', min: -3, max: 200, group: null },
  { key: 'floors_total', kind: 'integer', min: 1, max: 200, group: null },
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: null },
  { key: 'rooms', kind: 'enum', options: ['1', '2', '3', '4', '5', '6', '7_plus'], group: null },
  { key: 'entrance', kind: 'enum', options: ['separate', 'common'], group: null },
  { key: 'renovation', kind: 'enum', options: ['cosmetic', 'euro', 'design', 'required'], group: null },
];

const warehouseFields: AttrFieldList = [
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: null },
  { key: 'entrance', kind: 'enum', options: ['separate', 'common'], group: null },
  {
    key: 'building_type',
    kind: 'enum',
    options: ['business_center', 'warehouse_building', 'shopping_center', 'detached', 'residential'],
    group: null,
  },
];

const garageFields: AttrFieldList = [
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: null },
  { key: 'material', kind: 'enum', options: ['brick', 'metal', 'reinforced_concrete'], group: null },
];

const parkingFields: AttrFieldList = [
  { key: 'area_total', kind: 'number', unit: 'м²', min: 1, max: 100000, decimals: 1, group: null },
  { key: 'parking_location', kind: 'enum', options: ['underground', 'indoor', 'outdoor'], group: null },
  { key: 'parking_level', kind: 'integer', min: -5, max: 100, group: null },
  { key: 'spot_number', kind: 'string', maxLen: 50, group: null },
];

const landFields: AttrFieldList = [
  { key: 'land_area', kind: 'number', unit: 'сотки', min: 0.01, max: 1000000, decimals: 2, group: null },
  { key: 'land_type', kind: 'enum', options: ['izhs', 'garden', 'farm'], group: null },
];

export const catalog: Readonly<Record<PropertyType, AttrFieldList>> = {
  apartment: apartmentFields,
  apartments: apartmentFields,
  room: roomFields,
  house: houseFields,
  office: officeFields,
  commercial: officeFields,
  warehouse: warehouseFields,
  garage: garageFields,
  parking: parkingFields,
  land: landFields,
};

export const groupLabels: Readonly<Record<AttrGroup, string>> = {
  about_object: 'О квартире',
  about_building: 'О доме',
  about_house: 'О доме',
  about_land: 'Об участке',
};

export function fieldsForType(type: PropertyType): AttrFieldList {
  return catalog[type];
}

export function findField(type: PropertyType, key: AttrKey): AttrField | undefined {
  return fieldsForType(type).find((field) => field.key === key);
}

export function isPropertyType(value: unknown): value is PropertyType {
  return typeof value === 'string' && value in catalog;
}

export const fieldLabels: Readonly<Record<AttrKey, string>> = {
  rooms: 'Комнаты',
  area_total: 'Общая площадь',
  area_living: 'Жилая площадь',
  area_kitchen: 'Площадь кухни',
  floor: 'Этаж',
  floors_total: 'Этажность дома',
  bathroom: 'Санузел',
  balcony: 'Балкон',
  renovation: 'Ремонт',
  year_built: 'Год постройки',
  ceiling_height: 'Высота потолков',
  parking_type: 'Парковка',
  land_area: 'Площадь участка',
  land_type: 'Тип участка',
  house_type: 'Тип дома',
  material: 'Материал',
  shower: 'Душ',
  building_type: 'Тип здания',
  entrance: 'Вход',
  parking_location: 'Расположение',
  parking_level: 'Уровень',
  spot_number: 'Номер места',
};

export const enumLabels: Readonly<
  Partial<Record<AttrKey, Readonly<Record<string, string>>>>
> = {
  rooms: {
    studio: 'Студия',
    '1': '1',
    '2': '2',
    '3': '3',
    '4': '4',
    '5': '5',
    '6': '6',
    '7_plus': '7+',
  },
  bathroom: {
    combined: 'Совмещенный',
    separate: 'Раздельный',
    multiple: 'Несколько',
    indoor: 'В доме',
    outdoor: 'На улице',
    none: 'Нет',
  },
  balcony: {
    none: 'Нет',
    balcony: 'Балкон',
    loggia: 'Лоджия',
    balcony_and_loggia: 'Балкон и лоджия',
  },
  renovation: {
    cosmetic: 'Косметический',
    euro: 'Евро',
    design: 'Дизайнерский',
    required: 'Требуется',
  },
  parking_type: {
    closed: 'Закрытая',
    underground: 'Подземная',
    open: 'Открытая',
  },
  land_type: {
    izhs: 'ИЖС',
    garden: 'Садовый',
    farm: 'Фермерский',
  },
  house_type: {
    detached: 'Отдельный',
    part: 'Часть дома',
    townhouse: 'Таунхаус',
    duplex: 'Дуплекс',
  },
  material: {
    brick: 'Кирпичный',
    monolithic: 'Монолитный',
    panel: 'Панельный',
    brick_monolithic: 'Кирпично-монолитный',
    block: 'Блочный',
    wooden: 'Деревянный',
    reinforced_concrete: 'Железобетонный',
    metal: 'Металлический',
  },
  shower: {
    indoor: 'В доме',
    outdoor: 'На улице',
    none: 'Нет',
  },
  building_type: {
    business_center: 'Бизнес-центр',
    warehouse_building: 'Складское здание',
    shopping_center: 'Торговый центр',
    detached: 'Отдельное здание',
    residential: 'Жилой дом',
  },
  entrance: {
    separate: 'Отдельный',
    common: 'Общий',
  },
  parking_location: {
    underground: 'Подземная',
    indoor: 'Крытая',
    outdoor: 'Открытая',
  },
};

export type AttrCardItem = { readonly label: string; readonly value: string };

export function formatAttributeValue(
  type: PropertyType,
  key: AttrKey,
  value: string | number,
  attrs: PropertyAttributes,
): string {
  const def = findField(type, key);

  if (!def) {
    return String(value);
  }

  if (def.kind === 'enum') {
    return enumLabels[key]?.[String(value)] ?? String(value);
  }

  if (def.kind === 'number') {
    const num = typeof value === 'number' ? value : Number(value);
    const formatted = Number.isFinite(num)
      ? num.toFixed(def.decimals).replace('.', ',')
      : String(value);
    return `${formatted} ${def.unit}`;
  }

  if (def.kind === 'integer') {
    if (key === 'floor') {
      const floorsTotal = attrs.floors_total;
      if (floorsTotal !== undefined && (typeof floorsTotal === 'string' || typeof floorsTotal === 'number')) {
        return `${value} из ${floorsTotal}`;
      }
    }
    return String(value);
  }

  // def.kind === 'string'
  return String(value);
}

function isFilled(value: unknown): boolean {
  if (value === undefined) return false;
  if (value === null) return false;
  if (typeof value === 'string' && value === '') return false;
  return true;
}

export type AttrCardGroup = {
  readonly group: AttrGroup | null;
  readonly label: string | null;
  readonly items: readonly AttrCardItem[];
};

export function formatAttributesForCardGrouped(
  type: PropertyType,
  attrs: PropertyAttributes,
): AttrCardGroup[] {
  const hasFloor = isFilled(attrs.floor);
  const hasFloorsTotal = isFilled(attrs.floors_total);
  const hideFloorsTotal = hasFloor && hasFloorsTotal;

  type MutableGroup = { group: AttrGroup | null; label: string | null; items: AttrCardItem[] };
  const groups: MutableGroup[] = [];
  let currentGroup: AttrGroup | null | undefined;

  for (const field of fieldsForType(type)) {
    if (field.key === 'floors_total' && hideFloorsTotal) continue;

    const raw = attrs[field.key];
    if (!isFilled(raw)) continue;
    if (typeof raw !== 'string' && typeof raw !== 'number') continue;

    if (currentGroup !== field.group) {
      currentGroup = field.group;
      groups.push({
        group: field.group,
        label: field.group === null ? null : groupLabels[field.group],
        items: [],
      });
    }

    groups[groups.length - 1].items.push({
      label: fieldLabels[field.key],
      value: formatAttributeValue(type, field.key, raw, attrs),
    });
  }

  return groups;
}
