import type { PropertyType } from '@/entities/property';

export const propertyTypeLabels: Record<PropertyType, string> = {
  apartment: 'Квартира',
  room: 'Комната',
  apartments: 'Апартаменты',
  studio: 'Студия',
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

/**
 * Фразы автонейма по типу — дословное зеркало карты propertyNamePhrases
 * бэка (apps/backend/internal/properties/domain/property_name.go, #1001):
 * «Мой/Моя/Моё» + слово типа. Плейсхолдер инпута названия показывает
 * подсказку без серийника — серийник сервер считает сам при пустом имени.
 */
const propertyTypeAutonamePhrases: Record<PropertyType, string> = {
  apartment: 'Моя квартира',
  room: 'Моя комната',
  apartments: 'Мои апартаменты',
  studio: 'Моя студия',
  house: 'Мой дом',
  commercial: 'Моё коммерческое помещение',
  office: 'Мой офис',
  warehouse: 'Мой склад',
  garage: 'Мой гараж',
  parking: 'Моё машиноместо',
  land: 'Мой земельный участок',
};

/** Фраза автонейма типа для плейсхолдера; без типа — фолбэк бэка «Мой объект». */
export function propertyAutonamePhrase(type: PropertyType | undefined): string {
  return type === undefined ? 'Мой объект' : propertyTypeAutonamePhrases[type];
}
