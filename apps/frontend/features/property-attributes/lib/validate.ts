import type { PropertyType, PropertyAttributes } from '@/entities/property/model/types';
import { fieldsForType, findField } from '@/features/property-attributes/lib/catalog';
import type { AttrKey } from '@/features/property-attributes/model/attr-keys';

export type AttrErrors = Partial<Record<AttrKey, string>>;

export function filterByType(
  type: PropertyType,
  attrs: PropertyAttributes,
): PropertyAttributes {
  const fields = fieldsForType(type);
  const allowed = new Set<string>(fields.map((field) => field.key));
  const result: Record<string, string | number> = {};
  for (const [key, value] of Object.entries(attrs)) {
    if (allowed.has(key) && (typeof value === 'string' || typeof value === 'number')) {
      result[key] = value;
    }
  }
  return result;
}

function fractionDigits(value: string | number): number {
  const str = typeof value === 'number' ? String(value) : value;
  const dot = str.indexOf('.');
  if (dot === -1) return 0;
  return str.length - dot - 1;
}

function numericValue(
  attrs: PropertyAttributes,
  key: AttrKey,
): number | undefined {
  const value = attrs[key];
  if (typeof value === 'number') {
    return value;
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

export function validateField(
  type: PropertyType,
  key: AttrKey,
  value: string | number | undefined,
): string | undefined {
  const def = findField(type, key);
  if (!def) {
    return undefined;
  }
  if (value === undefined) {
    return undefined;
  }
  if (typeof value === 'string' && value === '') {
    return undefined;
  }

  if (def.kind === 'enum') {
    if (typeof value === 'string' && def.options.includes(value)) {
      return undefined;
    }
    return 'значение некорректно';
  }

  if (def.kind === 'number') {
    const parsed = typeof value === 'number' ? value : Number(value);
    if (Number.isNaN(parsed) || parsed < def.min || parsed > def.max) {
      return `от ${def.min} до ${def.max} ${def.unit}`;
    }
    if (fractionDigits(value) > def.decimals) {
      return `не более ${def.decimals} знаков после запятой`;
    }
    return undefined;
  }

  if (def.kind === 'integer') {
    const parsed = typeof value === 'number' ? value : Number(value);
    if (!Number.isInteger(parsed) || parsed < def.min || parsed > def.max) {
      return `от ${def.min} до ${def.max}`;
    }
    return undefined;
  }

  // def.kind === 'string'
  const str = typeof value === 'string' ? value : String(value);
  if (Array.from(str).length > def.maxLen) {
    return `не более ${def.maxLen} символов`;
  }
  return undefined;
}

export function validateAttributes(
  type: PropertyType,
  attrs: PropertyAttributes,
): AttrErrors {
  const errors: AttrErrors = {};
  const fields = fieldsForType(type);
  const fieldKeys = new Set<string>(fields.map((field) => field.key));

  for (const [rawKey, rawValue] of Object.entries(attrs)) {
    if (!fieldKeys.has(rawKey)) {
      continue;
    }
    if (typeof rawValue !== 'string' && typeof rawValue !== 'number') {
      continue;
    }
    const error = validateField(type, rawKey as AttrKey, rawValue);
    if (error !== undefined) {
      errors[rawKey as AttrKey] = error;
    }
  }

  // Cross-field rules: only applied when both sides present and individually valid.
  if (errors.floor === undefined && errors.floors_total === undefined) {
    const floor = numericValue(attrs, 'floor');
    const floorsTotal = numericValue(attrs, 'floors_total');
    if (floor !== undefined && floorsTotal !== undefined && floor > floorsTotal) {
      errors.floor = 'этаж не может превышать этажность';
    }
  }

  if (errors.area_living === undefined && errors.area_total === undefined) {
    const areaLiving = numericValue(attrs, 'area_living');
    const areaTotal = numericValue(attrs, 'area_total');
    if (areaLiving !== undefined && areaTotal !== undefined && areaLiving > areaTotal) {
      errors.area_living = 'не может превышать общую площадь';
    }
  }

  if (errors.area_kitchen === undefined && errors.area_total === undefined) {
    const areaKitchen = numericValue(attrs, 'area_kitchen');
    const areaTotal = numericValue(attrs, 'area_total');
    if (areaKitchen !== undefined && areaTotal !== undefined && areaKitchen > areaTotal) {
      errors.area_kitchen = 'не может превышать общую площадь';
    }
  }

  return errors;
}
