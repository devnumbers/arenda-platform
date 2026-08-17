import type { PropertyType, PropertyAttributes } from '@/entities/property';
import { type AttrGroup, fieldsForType, findField, groupLabels } from '@/features/property-attributes/lib/catalog';
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
    if (!isFilled(raw) || (typeof raw !== 'string' && typeof raw !== 'number')) {
      continue;
    }
    items.push({
      label: fieldLabels[field.key],
      value: formatAttributeValue(type, field.key, raw, attrs),
    });
  }
  return items;
}

export type AttrCardGroup = {
  readonly group: AttrGroup | null;
  readonly label: string | null;
  readonly items: readonly AttrCardItem[];
};

function isFilled(value: unknown): boolean {
  if (value === undefined) return false;
  if (value === null) return false;
  if (typeof value === 'string' && value === '') return false;
  return true;
}

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
