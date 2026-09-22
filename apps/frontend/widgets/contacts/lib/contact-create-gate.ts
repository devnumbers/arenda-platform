import type { Property } from '@/entities/property';
import { propertyPermissions } from '@/entities/property';

/**
 * Гейт объектной ветки создания контакта (#509): один взгляд на запрос
 * контекстного объекта решает, что показывает экран — скелетон, карточку
 * ошибки с «Повторить» или карточку недоступности. Отказ о правах звучит
 * только по успешно загруженному объекту: сетевая ошибка не имеет права
 * выглядеть как «доступ только для просмотра» (канон гейтов создания —
 * operation-create-wizard-screen / payment-create-wizard-screen: isError →
 * повтор, denial → лишь при загруженном объекте без права правки).
 * Структурный шов без React, как operationsFeedGate: принимает любые
 * query-результаты с isPending/isError/data.
 */
export type ContactCreateGate =
  | { readonly kind: 'pending' }
  | { readonly kind: 'error' }
  | { readonly kind: 'denied'; readonly archived: boolean }
  | { readonly kind: 'allowed' };

export function contactCreateGate(query: {
  readonly isPending: boolean;
  readonly isError: boolean;
  readonly data: Property | undefined;
}): ContactCreateGate {
  if (query.isPending) {
    return { kind: 'pending' };
  }
  // Ошибка и «объекта нет» — одно состояние экрана: повтор запроса. Вердикт
  // propertyPermissions(undefined) («всё закрыто») сюда не пролезает —
  // именно он превращал сетевой сбой в «только просмотр».
  if (query.isError || query.data === undefined) {
    return { kind: 'error' };
  }
  if (!propertyPermissions(query.data).canEdit) {
    return { kind: 'denied', archived: query.data.status === 'archived' };
  }
  return { kind: 'allowed' };
}
