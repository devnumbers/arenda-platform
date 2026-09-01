import type { components } from '@/shared/api/dto';
import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { propertyCreateStepReady, type PropertyCreateDraft } from './property-create-draft';

/**
 * Сабмит флоу «Создание объекта» (#482): команда POST /properties из
 * черновика. Характеристики уходят полной заменой только заполненными
 * ключами каталога текущего типа (бэкенд отклоняет чужие ключи);
 * «сырой» ввод нормализуется в проводные значения. Черновик при этом
 * чужие ключи хранит (lossless, docs/entities/obekt.md) — смена типа их
 * не удаляет и, по решению владельца, ничего не сообщает: экран просто
 * показывает каталог нового типа.
 *
 * Каталог характеристик — соседняя фича (features/property-attributes):
 * боковые импорты между фичами запрещены, поэтому фича принимает порт с
 * функциями каталога, а виджет инжектит реальные реализации (паттерн
 * resolveTitle из флоу платежей).
 */

type PropertyCreateRequest = components['schemas']['PropertyCreateRequest'];

/** Порт каталога характеристик: проводные значения, валидация и набор
 * ключей по типу. */
export type PropertyAttributesPort = {
  readonly toWireAttributes: (
    type: PropertyType,
    attrs: PropertyAttributes,
  ) => PropertyAttributes;
  readonly validateAttributes: (
    type: PropertyType,
    attrs: PropertyAttributes,
  ) => Partial<Record<string, string>>;
  readonly fieldKeys: (type: PropertyType) => ReadonlySet<string>;
};

/** Команда создания объекта из черновика; undefined — шаги не завершены
 * (тип/адрес/название) или характеристики не проходят каталог. */
export function buildPropertyCreateCommand(
  draft: PropertyCreateDraft,
  attributeCatalog: PropertyAttributesPort,
): PropertyCreateRequest | undefined {
  if (
    !propertyCreateStepReady(1, draft)
    || !propertyCreateStepReady(2, draft)
    || !propertyCreateStepReady(3, draft)
  ) {
    return undefined;
  }
  // Готовность шагов гарантирует заполненность обязательных полей —
  // сужаем типы для компилятора.
  const { type, address, name } = draft;
  if (type === undefined || address === undefined || name === undefined) {
    return undefined;
  }

  const attributes = attributeCatalog.toWireAttributes(type, draft.attributes ?? {});
  if (Object.keys(attributeCatalog.validateAttributes(type, attributes)).length > 0) {
    return undefined;
  }
  // Незаполненное и нечислимое wire отбрасывает; заполненное «сырое»
  // значение, не дожившее до payload, — некорректный ввод: команда не
  // строится, чтобы поле не пропало молча (ошибку показывает экран).
  const fieldKeys = attributeCatalog.fieldKeys(type);
  for (const [key, raw] of Object.entries(draft.attributes ?? {})) {
    if (typeof raw !== 'string' && typeof raw !== 'number') {
      continue;
    }
    const filled = typeof raw === 'string' ? raw.trim().length > 0 : true;
    if (filled && fieldKeys.has(key) && attributes[key] === undefined) {
      return undefined;
    }
  }

  const description = draft.description?.trim();
  return {
    name: name.trim(),
    type,
    address: address.trim(),
    ...(description !== undefined && description.length > 0 && { description }),
    ...(Object.keys(attributes).length > 0 && { attributes }),
  };
}

