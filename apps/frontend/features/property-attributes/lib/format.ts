import type { PropertyType, PropertyAttributes } from '@/entities/property/model/types';
import { fieldsForType, findField } from '@/features/property-attributes/lib/catalog';
import { fieldLabels, enumLabels } from '@/features/property-attributes/lib/labels';
import type { AttrKey } from '@/features/property-attributes/model/attr-keys';

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

export function formatAttributesForCard(
  type: PropertyType,
  attrs: PropertyAttributes,
): AttrCardItem[] {
  const items: AttrCardItem[] = [];
  for (const field of fieldsForType(type)) {
    const raw = attrs[field.key];
    if (raw === undefined) {
      continue;
    }
    if (typeof raw === 'string' && raw === '') {
      continue;
    }
    if (typeof raw !== 'string' && typeof raw !== 'number') {
      continue;
    }
    items.push({
      label: fieldLabels[field.key],
      value: formatAttributeValue(type, field.key, raw, attrs),
    });
  }
  return items;
}
