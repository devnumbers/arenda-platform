import type { FC, SVGProps } from 'react';

import {
  BoldBox,
  BoldBuild,
  BoldDoor,
  BoldFence,
  BoldGarage,
  BoldHome,
  BoldHouse,
  BoldLongSofa,
  BoldOfficeChair,
  BoldParking,
  BoldSofa,
} from '@/shared/assets/icons';

import type { PropertyType } from '../model/types';

/**
 * Реестр «тип объекта → глиф-плейсхолдер» по вариантам компонента
 * Category Icon (Figma 3099:151373…151403, карта #1217): Квартира —
 * Bold/Home, Студия — Bold/Sofa, Апартаменты — Bold/LongSofa, Комната —
 * Bold/Door, Дом — Bold/House, Коммерческое помещение — Bold/Build,
 * Офис — Bold/OfficeChair, Склад — Bold/Box, Гараж — Bold/Garage,
 * Машиноместо — Bold/Parking, Земельный участок — Bold/Fence. Круг и
 * канты поверхностей держат PropertyAvatar / CircleIcon, глиф — отсюда.
 */

type IconComponent = FC<SVGProps<SVGSVGElement>>;

/** Выборка по ключу — статичная ссылка из модуля (правило
 * react-hooks/static-components запрещает получать компонент вызовом
 * функции во время рендера). */
export const propertyTypeIcons: Readonly<Record<PropertyType, IconComponent>> = {
  apartment: BoldHome,
  apartments: BoldLongSofa,
  studio: BoldSofa,
  house: BoldHouse,
  room: BoldDoor,
  commercial: BoldBuild,
  office: BoldOfficeChair,
  warehouse: BoldBox,
  garage: BoldGarage,
  parking: BoldParking,
  land: BoldFence,
};
