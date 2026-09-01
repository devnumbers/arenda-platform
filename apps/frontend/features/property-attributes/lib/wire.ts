import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { type AttrField, fieldsForType } from './catalog';

/**
 * Проводное (wire) представление характеристик: значения черновика —
 * «сырой» ввод (строки, запятая как разделитель), в API уходят только
 * заполненные ключи каталога текущего типа числами и строками каталога
 * (docs/entities/obekt.md: полная замена, только заполненные ключи).
 */

/** Одно значение: числовые поля парсятся (запятая → точка, целые — без
 * дробной части), незаполненное и нечислимое отбрасывается. */
export function toWireAttribute(
  field: AttrField,
  value: string | number,
): string | number | undefined {
  if (field.kind === 'number' || field.kind === 'integer') {
    const raw = typeof value === 'string' ? value.trim().replace(',', '.') : value;
    // Пустая строка — незаполненное поле (Number('') = 0 ввёл бы ноль).
    if (typeof raw === 'string' && raw.length === 0) {
      return undefined;
    }
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) {
      return undefined;
    }
    if (field.kind === 'integer' && !Number.isInteger(parsed)) {
      return undefined;
    }
    return parsed;
  }
  const str = String(value).trim();
  return str.length > 0 ? str : undefined;
}

/** Заполненные ключи каталога текущего типа в проводном виде; чужие ключи
 * прежнего типа и пустые значения не попадают в payload. */
export function toWireAttributes(
  type: PropertyType,
  attrs: PropertyAttributes,
): PropertyAttributes {
  const fields = new Map<string, AttrField>(
    fieldsForType(type).map((field) => [field.key, field]),
  );
  const wire: Record<string, string | number> = {};
  for (const [key, raw] of Object.entries(attrs)) {
    const field = fields.get(key);
    if (field === undefined || (typeof raw !== 'string' && typeof raw !== 'number')) {
      continue;
    }
    const value = toWireAttribute(field, raw);
    if (value !== undefined) {
      wire[key] = value;
    }
  }
  return wire;
}
