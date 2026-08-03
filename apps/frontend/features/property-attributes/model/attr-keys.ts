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

export const attrKeys: readonly AttrKey[] = [
  'rooms',
  'area_total',
  'area_living',
  'area_kitchen',
  'floor',
  'floors_total',
  'bathroom',
  'balcony',
  'renovation',
  'year_built',
  'ceiling_height',
  'parking_type',
  'land_area',
  'land_type',
  'house_type',
  'material',
  'shower',
  'building_type',
  'entrance',
  'parking_location',
  'parking_level',
  'spot_number',
] as const;
