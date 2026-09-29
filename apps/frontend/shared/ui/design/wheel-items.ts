import type { WheelPickerItem } from './wheel-picker';

/** Колесо времени из length паддингованных предметов: '00', '01', … —
 * value совпадает с меткой ряда. Это контракт самого WheelPicker: выбор
 * идёт findIndex по value (wheel-picker.tsx), значение, не равное метке,
 * колесо не находит и садится на первый ряд (находка приёмки #810).
 * Паддинг двузначный — часы paddedItems(24), минуты paddedItems(60). */
export function paddedItems(length: number): ReadonlyArray<WheelPickerItem> {
  return Array.from({ length }, (_, value) => {
    const label = String(value).padStart(2, '0');
    return { value: label, label };
  });
}
