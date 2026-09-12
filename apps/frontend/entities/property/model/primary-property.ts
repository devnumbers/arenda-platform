import type { Property } from './types';

/**
 * Канон «Основной объект» (CONTEXT.md, резолюция #584): основной выше
 * любого не-основного; среди основных — первый отмеченный (раньший
 * pinned_at) выше, повторная отметка время не меняет. Единственное место,
 * где живёт семантика pinned_at на фронте: сортировка хаба (#586) и
 * лендинг таба «Объекты» (карта #583) потребляют одну функцию.
 */
export function comparePrimaryProperty(a: Property, b: Property): number {
  if (a.pinned_at && b.pinned_at) {
    return a.pinned_at < b.pinned_at ? -1 : a.pinned_at > b.pinned_at ? 1 : 0;
  }
  if (a.pinned_at) return -1;
  if (b.pinned_at) return 1;
  return 0;
}
