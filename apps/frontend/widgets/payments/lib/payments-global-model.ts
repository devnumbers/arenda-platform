/**
 * Модель секций главного экрана «Платежи» (карта #573, тикет #578; макеты
 * 879:9679/880:17866): фид #575 режется на секции «Избранные» и
 * «Просроченные», счётчики целого скоупа подписывают замыкающие карточки
 * «Все …». Состав секций и их порядок — решения владельца #574 (секции
 * «На оплату» в срезе нет).
 */

import { formatDayMonthWithYear, formatOverdueDays } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import type { GlobalPayment, GlobalPaymentObject } from '@/entities/payment';
import { globalOverdueList } from './overdue-global-model';
import { PAYMENT_COMPLETED_LABEL } from './payment-row-subtitle';

/** «+» создания платежа на глобальном хабе (решение владельца 23.10.2026):
 * видна, когда есть хоть один редактируемый объект (canon #703 — у чистого
 * зрителя визард упёрся бы в отказ сервера); ведёт в модалку выбора типа
 * «Платёж / Автоплатёж», из неё — в глобальный визард /payments/new
 * (аналог /operations/new #570), где объект выбирается шагом «Выбрать
 * объект». Черновой фазы у хаба больше нет — черновики живут только в
 * объектном входе (#453). id — уже отфильтрованные по праву правки
 * объекты. */
export function paymentHubCanCreate(editableIds: ReadonlyArray<string>): boolean {
  return editableIds.length > 0;
}

/** Замыкающая карточка секции хаба — «Все избранные», «Все просроченные»,
 * «Показать все» (решение владельца #1339): видна только когда в секции
 * три и больше карточек; одна-две обходятся без замыкания — страница
 * категории остаётся доступной с шеврона заголовка. Правило единообразное
 * для всех секций; будущая «На оплату» (#1251) наследует его. */
export function paymentHubSectionHasAllCard(cards: number): boolean {
  return cards >= 3;
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
 * без следующего вхождения строки нет (879:17555, Show Description=false).
 * Год — вне текущего (правило владельца 13.10): today — серверное поле
 * контракта (ADR 0048), «Ближайший» бывает и в январе. Отображаемая
 * завершённость фида (#1341) — «Полностью оплачен»: сервер разрешает
 * исключение открытой паузы на своей стороне, флаг строки отображению
 * точно соответствует. */
export function nearestDateLine(item: GlobalPayment): string | undefined {
  if (item.isCompleted) {
    return PAYMENT_COMPLETED_LABEL;
  }
  return item.nearestDate === null
    ? undefined
    : formatDayMonthWithYear(item.nearestDate, item.today);
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
