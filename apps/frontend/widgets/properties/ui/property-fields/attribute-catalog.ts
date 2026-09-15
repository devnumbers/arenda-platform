import { fieldsForType, toWireAttributes, validateAttributes } from '@/features/property-attributes';
import type { PropertyAttributesPort } from '@/features/properties';

/**
 * Порт каталога характеристик для форм объекта: реализации соседней фичи
 * собираются здесь (виджету доступны обе фичи — паттерн инъекции порта из
 * property-create-submit) и переиспользуются визардом создания и формой
 * правки (#590).
 */
export const attributeCatalog: PropertyAttributesPort = {
  toWireAttributes,
  validateAttributes,
  fieldKeys: (type) => new Set(fieldsForType(type).map((field) => field.key)),
};
