/**
 * Модель режима правки «Избранных платежей» (карта #573, тикет #579;
 * макеты 693:5903/889:25522): черновик порядка для dnd (#813) плюс
 * выделение для удаления (#814 — клик на десктопе, long-press и тапы на
 * таче, trash в навбаре). Сохранение порядка — полный список оставшихся
 * id в порядке черновика (PUT /payments/favorites/order — полное
 * замещение, плотные 1-based позиции строит сервер); удаление выбранных —
 * PUT favorite=false по каждому (тикет #814).
 */

import type { GlobalPayment } from '@/entities/payment';
import { pluralize } from '@/shared/lib/pluralize';

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

/** Заголовок навбара при активном выделении (макет 954-51409: «Выбрано
 * 3 платежа»): безличное «Выбрано» + склонение «платёж/платежа/платежей»
 * по правилам русского числительного. */
export function selectedFavoritesTitle(count: number): string {
  return `Выбрано ${count} ${pluralize(count, 'платёж', 'платежа', 'платежей')}`;
}

/** Переключение выделения строки (тап после включения режима выбора,
 * клик на десктопе). */
export function toggleFavoriteSelection(
  ids: ReadonlySet<string>,
  id: string,
): ReadonlySet<string> {
  const next = new Set(ids);
  if (next.has(id)) {
    next.delete(id);
  } else {
    next.add(id);
  }
  return next;
}

/** Выделить строку, не снимая уже выбранных (long-press выделяет —
 * никогда не развыделяет, тикет #814). */
export function selectFavoriteSelection(
  ids: ReadonlySet<string>,
  id: string,
): ReadonlySet<string> {
  const next = new Set(ids);
  next.add(id);
  return next;
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

/** Есть ли несохранённые изменения порядка: перестановка против исходного.
 * Удаление в правке применяется сразу trash'ем (#814), в дифф «Сохранить»
 * не входит — гасит кнопку без перестановки. */
export function hasFavoritesEdits(
  order: ReadonlyArray<GlobalPayment>,
  initialOrder: ReadonlyArray<GlobalPayment>,
): boolean {
  return order.some((item, index) => item.id !== initialOrder[index]?.id);
}
