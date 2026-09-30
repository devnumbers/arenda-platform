import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { coerceAttributes } from '@/entities/property';
import { propertyTypeOptions } from './property-types';

/**
 * Модель флоу «Создание объекта» в новом дизайне (#480): три шага —
 * 1 «Выбор категории» (Figma 1213-52111), 2 «Адрес» (#481, Figma
 * 1213-52017/52391, 1519-94336), 3 «Характеристики» (#482, Figma
 * 1218-54295). Экран успеха шагом не считается — он появляется после
 * успешного POST и черновика не хранит (#483, Figma 1425-55788).
 * Позиция шага в черновике не живёт — флоу выводит её из заполненных
 * полей (как в визарде платежей #464).
 */

export type PropertyCreateStep = 1 | 2 | 3;

export const PROPERTY_CREATE_TOTAL_STEPS = 3;

export type PropertyCreateDraft = {
  /** Категория объекта (шаг 1). Тип apartments на шаге 1 не выбирается —
   * он появляется на шаге 3 как «Тип жилья» внутри категории «Квартира»
   * (Figma 1218-54295), поэтому валидатор его пропускает, а чипы шага 1
   * его не предлагают. */
  readonly type?: PropertyType;
  /** Адрес объекта (шаг 2). */
  readonly address?: string;
  /** Название объекта (шаг 3; обязательное поле домена). */
  readonly name?: string;
  /** Описание объекта (шаг 3; необязательно). */
  readonly description?: string;
  /** Характеристики из каталога по типу (шаг 3; все необязательные). */
  readonly attributes?: PropertyAttributes;
};

export const PROPERTY_CREATE_DRAFT_STORAGE_KEY = 'property-create-draft';

export const DEFAULT_PROPERTY_CREATE_DRAFT: PropertyCreateDraft = {};

/** Чипы шага 1 (Figma 1213-52111): девять категорий макета — все типы
 * домена, кроме apartments (см. PropertyCreateDraft.type). Порядок и
 * подписи — из общего реестра типов, лейбл домена вместо опечатки макета
 * («Земельныый участок»). */
export const propertyCategoryOptions: readonly { value: PropertyType; label: string }[] =
  propertyTypeOptions.filter((option) => option.value !== 'apartments');

/** Чип-группа «Тип жилья» шага 3 (Figma 1218-54295): закрывает тип
 * apartments внутри категории «Квартира» — оба значения делят один
 * каталог характеристик, поэтому смена между ними не меняет набор полей. */
export const propertyHousingTypeOptions: readonly { value: PropertyType; label: string }[] = [
  { value: 'apartment', label: 'Квартира' },
  { value: 'apartments', label: 'Апартаменты' },
];

/** Категория «Квартира» шага 1 (апартаменты появляются только здесь, на
 * шаге «Тип жилья»). */
export function isApartmentCategory(type: PropertyType): boolean {
  return type === 'apartment' || type === 'apartments';
}

/** Готовность шага: категория выбрана / адрес непустой / шаг 3 достигнут.
 * Характеристики, описание и название на готовность шага 3 не влияют —
 * они необязательные: пустое название регенерирует бэк из типа
 * («Моя квартира 1», #1001). */
export function propertyCreateStepReady(
  step: PropertyCreateStep,
  draft: PropertyCreateDraft,
): boolean {
  switch (step) {
    case 1:
      return draft.type !== undefined;
    case 2:
      return draft.address !== undefined && draft.address.trim().length > 0;
    case 3:
      return true;
  }
}

/** Восстановление черновика: первый незавершённый шаг флоу. */
export function initialPropertyCreateStep(draft: PropertyCreateDraft): PropertyCreateStep {
  if (!propertyCreateStepReady(1, draft)) return 1;
  if (!propertyCreateStepReady(2, draft)) return 2;
  return 3;
}

function isFilledString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

/** Форма-проверка persisted payload: неизвестный тип роняет весь черновик
 * (мусор из хранилища не всплывает в визарде), пустые строки и
 * нескалярные атрибуты отбрасываются. Поле step черновиков старого
 * визарда игнорируется — его поля продолжают жить в новом флоу, позицию
 * шага выводит initialPropertyCreateStep. */
export function validatePropertyCreateDraft(parsed: unknown): PropertyCreateDraft {
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return DEFAULT_PROPERTY_CREATE_DRAFT;
  }

  const record = parsed as Record<string, unknown>;

  const isValidType = (value: unknown): value is PropertyType =>
    propertyTypeOptions.some((option) => option.value === value);
  if ('type' in record && record.type !== undefined && !isValidType(record.type)) {
    return DEFAULT_PROPERTY_CREATE_DRAFT;
  }

  const address = isFilledString(record.address) ? record.address : undefined;
  const name = isFilledString(record.name) ? record.name : undefined;
  const description = isFilledString(record.description) ? record.description : undefined;
  const attributesPresent = 'attributes' in record && record.attributes !== undefined;

  return {
    ...(record.type !== undefined && { type: record.type as PropertyType }),
    ...(address !== undefined && { address }),
    ...(name !== undefined && { name }),
    ...(description !== undefined && { description }),
    ...(attributesPresent && { attributes: coerceAttributes(record.attributes) }),
  };
}
