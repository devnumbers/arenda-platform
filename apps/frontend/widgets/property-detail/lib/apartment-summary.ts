import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { formatAttributeValue } from '@/features/property-attributes';
import { pluralize } from '@/shared/lib/pluralize';

/**
 * Строки сводки «Квартира» на детали объекта (тикет #589, Figma
 * 1185:40820): Комнат / Общая площадь / Этаж / Ремонт — только
 * заполненные характеристики, в порядке макета. Значения считает
 * канонный форматировщик характеристик (этаж — «12 из 16», ремонт —
 * словарь); комнаты — с согласованием («2 комнаты»).
 */
export type ApartmentSummaryRow = {
  readonly label: string;
  readonly value: string;
};

function filled(value: unknown): value is string | number {
  if (value === undefined || value === null) return false;
  if (typeof value === 'string' && value === '') return false;
  return true;
}

export function propertyApartmentSummaryRows(
  type: PropertyType,
  attrs: PropertyAttributes,
): ReadonlyArray<ApartmentSummaryRow> {
  const rows: ApartmentSummaryRow[] = [];

  const rooms = attrs.rooms;
  if (typeof rooms === 'number') {
    rows.push({
      label: 'Комнат',
      value: `${rooms} ${pluralize(rooms, 'комната', 'комнаты', 'комнат')}`,
    });
  }

  const area = attrs.area_total;
  if (filled(area)) {
    rows.push({
      label: 'Общая площадь',
      value: formatAttributeValue(type, 'area_total', area, attrs),
    });
  }

  const floor = attrs.floor;
  if (filled(floor)) {
    rows.push({
      label: 'Этаж',
      value: formatAttributeValue(type, 'floor', floor, attrs),
    });
  }

  const renovation = attrs.renovation;
  if (filled(renovation)) {
    rows.push({
      label: 'Ремонт',
      value: formatAttributeValue(type, 'renovation', renovation, attrs),
    });
  }

  return rows;
}
