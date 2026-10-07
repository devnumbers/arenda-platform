/**
 * Модель секций главного экрана «Платежи» (карта #573, тикет #578; макеты
 * 879:9679/880:17866): фид #575 режется на секции «Избранные» и
 * «Просроченные», счётчики целого скоупа подписывают замыкающие карточки
 * «Все …». Состав секций и их порядок — решения владельца #574 (секции
 * «На оплату» в срезе нет).
 */

import { formatDayMonth, formatOverdueDays } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import type { GlobalPayment, GlobalPaymentObject } from '@/entities/payment';
import { globalOverdueList } from './overdue-global-model';

/** Цель «+» создания платежа на глобальном хабе (макет 3226-75059,
 * тикет #1235): единственный редактируемый объект — шит единого входа
 * (#453: черновик → выбор типа → визард его объекта, как у CTA пустых
 * «Объектов» #1066); несколько — страница «Объекты» (существующий выбор
 * объекта: карточка → платежи объекта → «Добавить»); ни одного — «+» нет
 * (canon #703: у чистого зрителя визард упёрся бы в 403). Глобального
 * роута создания (аналог /operations/new #570) у платежей нет — новый
 * вход не заводим. id — уже отфильтрованные по праву правки объекты. */
export type PaymentHubAddTarget =
  | { readonly kind: 'sheet'; readonly propertyId: string }
  | { readonly kind: 'objects' };

export function paymentHubAddTarget(
  editableIds: ReadonlyArray<string>,
): PaymentHubAddTarget | null {
  const [single] = editableIds;
  if (single === undefined) {
    return null;
  }
  return editableIds.length === 1
    ? { kind: 'sheet', propertyId: single }
    : { kind: 'objects' };
}

/** Избранные правила: звезда, порядок — сохранённые позиции favoriteOrder
 * (#576), никогда не упорядочивавшиеся (null) — в конец («новое избранное —
 * в конец»); между собой — порядок фида (сортировка стабильна). */
export function globalFavoritePayments(
  items: ReadonlyArray<GlobalPayment>,
): ReadonlyArray<GlobalPayment> {
  return items
    .filter((item) => item.isFavorite)
    .sort(
      (a, b) =>
        (a.favoriteOrder ?? Number.POSITIVE_INFINITY) -
        (b.favoriteOrder ?? Number.POSITIVE_INFINITY),
    );
}

/** Просроченные правила: накопленная просрочка — planned-вхождения в
 * прошлом. Порядок — по возрасту просрочки, старейшие первыми (решение
 * владельца 09.09) — направление «Старые» страницы просрочки (#580). */
export function globalOverduePayments(
  items: ReadonlyArray<GlobalPayment>,
): ReadonlyArray<GlobalPayment> {
  return globalOverdueList(items, 'old');
}

/** Счётчик карточки «Все избранные»: «17 платежей». */
export function paymentsCountLabel(count: number): string {
  return `${count} ${pluralize(count, 'платёж', 'платежа', 'платежей')}`;
}

/** Счётчик карточки «Все просроченные» — просроченные ОПЕРАЦИИ скоупа:
 * «7 просроченных». */
export function overdueOperationsCountLabel(count: number): string {
  return `${count} ${pluralize(count, 'просроченный', 'просроченных', 'просроченных')}`;
}

/** Дата-«Ближайший» (699:9446): просто дата графика, без статуса; у правила
 * без следующего вхождения строки нет (879:17555, Show Description=false). */
export function nearestDateLine(item: GlobalPayment): string | undefined {
  return item.nearestDate === null ? undefined : formatDayMonth(item.nearestDate);
}

/** Срок просрочки под суммой: возраст старейшего вхождения — «2 дня». */
export function overdueDaysLine(item: GlobalPayment): string | undefined {
  return item.overdueDays === null ? undefined : formatOverdueDays(item.overdueDays);
}

/** Флаг красной точки на карточке объекта (#575): просрочка в любой из
 * стопок — «Автоплатежи» или «Платежи». */
export function globalPaymentObjectHasOverdue(object: GlobalPaymentObject): boolean {
  return (
    object.autoPayRules.some((key) => key.hasOverdue) ||
    object.otherRules.some((key) => key.hasOverdue)
  );
}
