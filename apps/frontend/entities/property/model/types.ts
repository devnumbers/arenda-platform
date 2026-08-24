import type { AccessRole } from '@/shared/model/access';

export type PropertyStatus = 'active' | 'maintenance' | 'archived';
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

export type PropertyAttributes = Readonly<Record<string, string | number>>;

/** Контекст доступа актора к объекту: роль и имя владельца (только для чужих объектов). */
export type PropertyAccess = {
  readonly role: AccessRole;
  readonly ownerName?: string;
};

export type Property = {
  readonly id: string;
  readonly name: string;
  readonly type: PropertyType;
  readonly address: string;
  readonly description?: string;
  readonly attributes: PropertyAttributes;
  readonly status: PropertyStatus;
  readonly photos?: PropertyPhoto[];
  readonly access?: PropertyAccess;
  readonly members_count: number;
};
