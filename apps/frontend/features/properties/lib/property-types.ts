import type { PropertyType } from '@/entities/property';

export const propertyTypeLabels: Record<PropertyType, string> = {
  apartment: 'Квартира',
  room: 'Комната',
  apartments: 'Апартаменты',
  house: 'Дом',
  commercial: 'Коммерческое помещение',
  office: 'Офис',
  warehouse: 'Склад',
  garage: 'Гараж',
  parking: 'Машиноместо',
  land: 'Земельный участок',
};

export const propertyTypeOptions: { value: PropertyType; label: string }[] =
  Object.entries(propertyTypeLabels).map(([value, label]) => ({
    value: value as PropertyType,
    label,
  }));
