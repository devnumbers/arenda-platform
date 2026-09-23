/**
 * Модель режима правки «Избранных платежей» (карта #573, тикет #579;
 * макеты 693:5903/889:25522): черновик порядка для ручного dnd (#576)
 * плюс пометки на удаление (крест-звезда строки). Сохранение — PUT
 * favorite=false по каждому помеченному и полный список оставшихся id в
 * порядке черновика (PUT /payments/favorites/order — полное замещение,
 * плотные 1-based позиции строит сервер).
 */

import type { GlobalPayment } from '@/entities/payment';

/** Перестановка в черновике: элемент с позиции `from` встаёт на `to`,
 * соседи сдвигаются. Невалидные индексы (вне списка) возвращают исходный
 * массив — dnd-ручка не двигает строку мимо краёв. */
export function moveFavorite(
  items: ReadonlyArray<GlobalPayment>,
  from: number,
  to: number,
): ReadonlyArray<GlobalPayment> {
  if (from < 0 || to < 0 || from >= items.length || to >= items.length) {
    return items;
  }
  const next = [...items];
  const [moved] = next.splice(from, 1);
  if (moved === undefined) {
    return items;
  }
  next.splice(to, 0, moved);
  return next;
}

/** idы сохраняемого порядка: черновик без помеченных на удаление. */
export function remainingFavoriteIds(
  order: ReadonlyArray<GlobalPayment>,
  removedIds: ReadonlySet<string>,
): string[] {
  return order.filter((item) => !removedIds.has(item.id)).map((item) => item.id);
}

/** Род события клавиатурного порядка: взятие ручки, перемещение стрелками
 * (и каждый такой шаг), отпускание. */
export type FavoriteDragAnnouncementKind = 'grab' | 'move' | 'release';

/** Текст для aria-live-анонса клавиатурного порядка: позиции 1-based
 * (человеческий счёт), title — название платежа. */
export function favoriteDragAnnouncement(
  kind: FavoriteDragAnnouncementKind,
  title: string,
  position: number,
  total: number,
): string {
  const at = `«${title}», позиция ${position} из ${total}`;
  switch (kind) {
    case 'grab':
      return `${at}. Стрелки вверх и вниз — переместить, пробел — отпустить.`;
    case 'move':
      return `${at}.`;
    case 'release':
      return `${at}. Изменения порядка применятся кнопкой «Сохранить».`;
  }
}

/** Есть ли несохранённые изменения правки: перестановка против исходного
 * порядка или хотя бы одна пометка. Гасит «Сохранить» без диффа. */
export function hasFavoritesEdits(
  order: ReadonlyArray<GlobalPayment>,
  initialOrder: ReadonlyArray<GlobalPayment>,
  removedIds: ReadonlySet<string>,
): boolean {
  if (removedIds.size > 0) {
    return true;
  }
  return order.some((item, index) => item.id !== initialOrder[index]?.id);
}
