import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { propertyTypeLabels } from './property-types';

/**
 * Модель флоу «Создание объекта» в новом дизайне (#480): три шага —
 * 1 «Выбор категории» (аменда владельца 22.10: Figma 3625-89189 —
 * иллюстрация над заголовком, 11 главных категорий), 2 «Адрес» (#481,
 * Figma 1213-52017/52391, 1519-94336), 3 «Характеристики» (#482, Figma
 * 1218-54295). Экран успеха шагом не считается — он появляется после
 * успешного POST (#483, Figma 1425-55788). Состояние шагов — обычный
 * useState потока (карта #1052, Q2=В: черновика у создания объекта нет —
 * уход со страницы, перезагрузка и закрытие дают чистый лист).
 *
 * Двухуровневой модели больше нет (аменда владельца 22.10): квартира,
 * студия и апартаменты — главные категории шага 1, поля «Тип жилья» у
 * них нет ни в создании, ни в правке.
 */

export type PropertyCreateStep = 1 | 2 | 3;

export const PROPERTY_CREATE_TOTAL_STEPS = 3;

export type PropertyCreateDraft = {
  /** Категория объекта (шаг 1) — тип домена выбирается сразу, один из
   * одиннадцати (аменда владельца 22.10, Figma 3625-89189). */
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

/** Чипы шага 1 (аменда владельца 22.10, Figma 3625-89189): все одиннадцать
 * типов домена главными категориями, порядок кадра — квартира, студия и
 * апартаменты сразу после квартиры. Порядок кадра ≠ порядок реестра
 * (propertyTypeOptions), поэтому список явный, а не фильтр: новый тип
 * реестра на шаг 1 не попадёт, пока не добавлен сюда. Подписи — из общего
 * реестра типов (лейбл домена вместо опечатки макета «Земельныый
 * участок»). */
export const propertyCategoryOptions: readonly { value: PropertyType; label: string }[] = ([
  'apartment',
  'studio',
  'apartments',
  'room',
  'house',
  'commercial',
  'office',
  'warehouse',
  'garage',
  'parking',
  'land',
] as const).map((value) => ({ value, label: propertyTypeLabels[value] }));

/** Готовность шага: категория выбрана / адрес непустой / шаг 3 достигнут.
 * Характеристики, описание и название на готовность шага 3 не влияют —
 * они необязательные: пустое название регенерирует бэк из типа
 * («Моя квартира» — первый объект типа без серийника, #1001/#1078). */
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
