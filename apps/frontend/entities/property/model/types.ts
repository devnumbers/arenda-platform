export type PropertyStatus = 'active' | 'maintenance' | 'archived';
export type Occupancy = 'free' | 'occupied';
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

export type PropertyPhoto = {
  readonly id: string;
  readonly url: string;
};

export type Property = {
  readonly id: string;
  readonly name: string;
  readonly type: PropertyType;
  readonly address: string;
  readonly description?: string;
  readonly status: PropertyStatus;
  readonly occupancy: Occupancy;
  readonly photos?: PropertyPhoto[];
};
