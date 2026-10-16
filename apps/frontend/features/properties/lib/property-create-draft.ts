import type { PropertyAttributes, PropertyType } from '@/entities/property';
import { propertyTypeOptions } from './property-types';

/**
 * Модель флоу «Создание объекта» в новом дизайне (#480): три шага —
 * 1 «Выбор категории» (Figma 1213-52111), 2 «Адрес» (#481, Figma
 * 1213-52017/52391, 1519-94336), 3 «Характеристики» (#482, Figma
 * 1218-54295). Экран успеха шагом не считается — он появляется после
 * успешного POST (#483, Figma 1425-55788). Состояние шагов — обычный
 * useState потока (карта #1052, Q2=В: черновика у создания объекта нет —
 * уход со страницы, перезагрузка и закрытие дают чистый лист).
 */

export type PropertyCreateStep = 1 | 2 | 3;

export const PROPERTY_CREATE_TOTAL_STEPS = 3;

export type PropertyCreateDraft = {
  /** Категория объекта (шаг 1). Тип apartments на шаге 1 не выбирается —
   * он появляется на шаге 3 как «Тип жилья» внутри категории «Квартира»
   * (Figma 1218-54295), поэтому чипы шага 1 его не предлагают. */
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

/** Чипы шага 1 (Figma 1213-52111): девять категорий макета — все типы
 * домена, кроме apartments (см. PropertyCreateDraft.type). Порядок и
 * подписи — из общего реестра типов, лейбл домена вместо опечатки макета
 * («Земельныый участок»). */
export const propertyCategoryOptions: readonly { value: PropertyType; label: string }[] =
  propertyTypeOptions.filter(
    (option) => option.value !== 'apartments' && option.value !== 'studio',
  );

/** Чип-группа «Тип жилья» шага 3 (Figma 1218-54295; студия добавлена по
 * решению владельца 30.09, тикет #1003): закрывают типы внутри категории
 * «Квартира» — все три делят один каталог характеристик (студия — клон
 * набора квартиры), поэтому смена между ними не меняет набор полей.
 * Порядок по кадру — Квартира, Студия, Апартаменты (тикет #1081). */
export const propertyHousingTypeOptions: readonly { value: PropertyType; label: string }[] = [
  { value: 'apartment', label: 'Квартира' },
  { value: 'studio', label: 'Студия' },
  { value: 'apartments', label: 'Апартаменты' },
];

/** Категория двухуровневой модели, представляющая тип в чип-пикерах
 * (шаг 1 создания, «Тип объекта» правки): все три типа жилья показываются
 * чипом «Квартира» — уточнение живёт в «Типе жилья» (#1003, #1081). */
export function propertyCategoryOf(type: PropertyType): PropertyType {
  return isApartmentCategory(type) ? 'apartment' : type;
}

/** Категория «Квартира» шага 1 (апартаменты и студия появляются только
 * здесь, на шаге «Тип жилья»). */
export function isApartmentCategory(type: PropertyType): boolean {
  return type === 'apartment' || type === 'apartments' || type === 'studio';
}

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
