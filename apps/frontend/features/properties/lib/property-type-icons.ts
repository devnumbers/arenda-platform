import type { FC, SVGProps } from 'react';

import type { PropertyType } from '@/entities/property';
import {
  BoldBox,
  BoldBuild,
  BoldCar,
  BoldFence,
  BoldHome,
  BoldKey,
  BoldPrinter,
  BoldSofa,
} from '@/shared/assets/icons';

/**
 * Иконки типа объекта на экране успеха визарда (Figma 1425-55788, #483):
 * серый глиф внутри светлого круга 96. Макет закрепляет «Квартиру» за
 * Icon/Bold/Home; остальные типы — из того же набора bold-иконок, что и
 * каталог платежей (#447): «Машиноместо» повторяет семантику каталога
 * («Парковка» — bold-car), «Дом» и «Апартаменты» делят единственную
 * «домашнюю» иконку набора с квартирой (категория «Квартира», #482),
 * офис читается офисной техникой (bold-printer).
 */

type IconComponent = FC<SVGProps<SVGSVGElement>>;

/** Реестр «тип объекта → иконка»: выборка по ключу — статичная ссылка
 * из модуля (правило react-hooks/static-components запрещает получать
 * компонент вызовом функции во время рендера). */
export const propertyTypeIcons: Readonly<Record<PropertyType, IconComponent>> = {
  apartment: BoldHome,
  apartments: BoldHome,
  studio: BoldSofa,
  house: BoldHome,
  room: BoldSofa,
  commercial: BoldBuild,
  office: BoldPrinter,
  warehouse: BoldBox,
  garage: BoldKey,
  parking: BoldCar,
  land: BoldFence,
};
