/**
 * Выбор объектов мультиобъектного приглашения (карта #692, тикет #699;
 * макеты 2008-46375 / 2008-46627 / 2008-80773): чистая математика —
 * tri-state чекбокса «Все объекты», тапы по строкам и свёрнутая сводка
 * экрана приглашения. «Все объекты» — снапшот текущих активных объектов
 * (решение чарта карты #692): будущие автоматически не шарятся.
 */

/** Состояние чекбокса «Все объекты»: off / mixed / on в терминах макета. */
export type InviteSelectionState = 'all' | 'partial' | 'none';

/** Дефолт экрана приглашения — «Все N объектов»: чекает все текущие
 * активные объекты читающего. */
export function allInvitedProperties(optionIds: readonly string[]): ReadonlySet<string> {
  return new Set(optionIds);
}

export function inviteSelectionState(
  selected: ReadonlySet<string>,
  optionIds: readonly string[],
): InviteSelectionState {
  if (selected.size === 0) {
    return 'none';
  }
  return isAllSelected(selected, optionIds) ? 'all' : 'partial';
}

/** «Выбраны все опции»: ноль опций всем selections не считается. */
function isAllSelected(
  selected: ReadonlySet<string>,
  optionIds: readonly string[],
): boolean {
  return optionIds.length > 0 && selected.size === optionIds.length;
}

/** Тап по «Все объекты»: полный выбор снимает всё, иначе чекает все. */
export function toggleAllInvitedProperties(
  selected: ReadonlySet<string>,
  optionIds: readonly string[],
): ReadonlySet<string> {
  return isAllSelected(selected, optionIds) ? new Set() : new Set(optionIds);
}

export function toggleInvitedProperty(
  selected: ReadonlySet<string>,
  propertyId: string,
): ReadonlySet<string> {
  const next = new Set(selected);
  if (next.has(propertyId)) {
    next.delete(propertyId);
  } else {
    next.add(propertyId);
  }
  return next;
}

/** Строка свёрнутой сводки: либо агрегатная «Все N объектов», либо
 * конкретный объект с фото и адресом. */
export type CollapsedInviteRow<T> =
  | { readonly kind: 'all' }
  | { readonly kind: 'property'; readonly option: T };

/**
 * Свёрнутая сводка экрана приглашения: полный выбор — одна агрегатная
 * строка «Все N объектов / Поделиться всеми объектами» (2008-80773),
 * иначе — по строке на выбранный объект в порядке списка (2008-46375);
 * пустой выбор сводки не рисует.
 */
export function collapsedInviteRows<T extends { readonly id: string }>(
  options: ReadonlyArray<T>,
  selected: ReadonlySet<string>,
): ReadonlyArray<CollapsedInviteRow<T>> {
  if (selected.size === 0) {
    return [];
  }
  if (isAllSelected(selected, options.map((option) => option.id))) {
    return [{ kind: 'all' }];
  }
  return options
    .filter((option) => selected.has(option.id))
    .map((option) => ({ kind: 'property' as const, option }));
}
