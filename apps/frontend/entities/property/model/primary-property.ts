import type { Property } from './types';

/**
 * Канон «Основной объект» (CONTEXT.md, резолюция #584): основной выше
 * любого не-основного; среди основных — первый отмеченный (раньший
 * pinned_at) выше, повторная отметка время не меняет. Единственное место,
 * где живёт семантика pinned_at на фронте: сортировка хаба (#586),
 * лендинг таба «Объекты» (карта #583) и порядок/предвыбор шага
 * «Выбрать объект» визардов (решение владельца 23.10.2026) потребляют
 * одни функции.
 */
export function comparePrimaryProperty(a: Property, b: Property): number {
  if (a.pinned_at && b.pinned_at) {
    return a.pinned_at < b.pinned_at ? -1 : a.pinned_at > b.pinned_at ? 1 : 0;
  }
  if (a.pinned_at) return -1;
  if (b.pinned_at) return 1;
  return 0;
}

/** Список в канонном порядке «Основного объекта»: основные выше,
 * не-основные сохраняют исходный порядок (стабильная сортировка).
 * Исходный массив не мутирует. */
export function propertiesByPrimaryOrder<T extends Property>(
  properties: ReadonlyArray<T>,
): ReadonlyArray<T> {
  return [...properties].sort(comparePrimaryProperty);
}

/** Объект, выбранный на шаге «Выбрать объект» (визарды платежа и
 * операции, решение владельца 23.10.2026): явный выбор пользователя
 * сильнее; до явного — первый по канону «Основного объекта», то есть
 * верхняя строка списка шага (прецедент effectivePaymentType: в черновике
 * хранится только явный выбор). undefined — редактируемых объектов нет. */
export function effectiveSelectedPropertyId(
  selectedId: string | undefined,
  properties: ReadonlyArray<Property>,
): string | undefined {
  if (selectedId !== undefined) {
    return selectedId;
  }
  return propertiesByPrimaryOrder(properties)[0]?.id;
}
