import type { components } from '@/shared/api/dto';
import { coerceAttributes } from '@/entities/property';
import type { Property, PropertyAttributes, PropertyType } from '@/entities/property';
import type { PropertyAttributesPort } from './property-create-submit';

export type { PropertyAttributesPort };

/**
 * Сабмит флоу «Редактирование объекта» (карта #583, тикет #590): команда
 * PATCH /properties/{id} из черновика формы. Контракт тот же, что у
 * создания (property-create-submit): черновик хранит чужие ключи прежнего
 * типа lossless, в команду уходят только ключи каталога текущего типа в
 * проводном виде; «сырое» заполненное значение, не дожившее до payload,
 * блокирует команду, чтобы поле не пропало молча.
 */

type PropertyUpdateRequest = components['schemas']['PropertyUpdateRequest'];

/** Черновик формы правки: атрибуты хранятся «как набрано» — числа из
 * объекта остаются числами, правки пользователя приходят строками. */
export type PropertyEditDraft = {
  readonly type?: PropertyType;
  readonly name: string;
  readonly address: string;
  readonly description: string;
  readonly attributes: PropertyAttributes;
};

/** Черновик, повторяющий загруженный объект, — стартовое состояние формы. */
export function initialPropertyEditDraft(property: Property): PropertyEditDraft {
  return {
    type: property.type,
    name: property.name,
    address: property.address,
    description: property.description ?? '',
    attributes: coerceAttributes(property.attributes),
  };
}

/** Команда правки объекта; undefined — обязательные поля пусты или
 * характеристики не проходят каталог. */
export function buildPropertyEditCommand(
  draft: PropertyEditDraft,
  attributeCatalog: PropertyAttributesPort,
): PropertyUpdateRequest | undefined {
  const { type } = draft;
  const name = draft.name.trim();
  const address = draft.address.trim();
  if (type === undefined || name.length === 0 || address.length === 0) {
    return undefined;
  }

  const attributes = attributeCatalog.toWireAttributes(type, draft.attributes);
  if (Object.keys(attributeCatalog.validateAttributes(type, attributes)).length > 0) {
    return undefined;
  }
  const fieldKeys = attributeCatalog.fieldKeys(type);
  for (const [key, raw] of Object.entries(draft.attributes)) {
    if (typeof raw !== 'string' && typeof raw !== 'number') {
      continue;
    }
    const filled = typeof raw === 'string' ? raw.trim().length > 0 : true;
    if (filled && fieldKeys.has(key) && attributes[key] === undefined) {
      return undefined;
    }
  }

  return {
    name,
    type,
    address,
    description: draft.description.trim() || undefined,
    attributes,
  };
}

/** Равенство наборов атрибутов: те же ключи с теми же значениями. */
export function attributesEqual(
  a: PropertyAttributes,
  b: PropertyAttributes,
): boolean {
  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  if (aKeys.length !== bKeys.length) {
    return false;
  }
  return aKeys.every((key) => a[key] === b[key]);
}

/** Есть ли у черновика изменения против объекта: сравнивается то, что
 * ушло бы в команду, — обрезка пробелов и пустое описание изменением не
 * считаются. Невалидный черновик сравнению не подлежит (кнопка сохранения
 * и так погашена готовностью). */
export function propertyEditDirty(
  draft: PropertyEditDraft,
  property: Property,
  attributeCatalog: PropertyAttributesPort,
): boolean {
  const command = buildPropertyEditCommand(draft, attributeCatalog);
  if (command === undefined) {
    return false;
  }
  return (
    command.name !== property.name
    || command.type !== property.type
    || command.address !== property.address
    || command.description !== (property.description ?? undefined)
    || !attributesEqual(coerceAttributes(command.attributes), coerceAttributes(property.attributes))
  );
}
