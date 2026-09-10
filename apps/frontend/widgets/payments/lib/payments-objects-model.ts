/**
 * Модель страницы «Объекты» глобальных платежей (карта #573, тикет #582;
 * макет 654:7558): стопки карточки объекта из данных #575, обрезка по
 * решениям владельца 10.09 — при обеих группах максимум 4 иконки в группе,
 * при одной — 7, без счётчика; пустые группы не показываются. Иконки ключей
 * — категории правил фида (#575) по paymentId: объектный ответ ключей
 * категорий не несёт.
 */

import { categoryStyle } from '@/features/payment-categories';
import type {
  GlobalPayment,
  GlobalPaymentObject,
} from '@/entities/payment';

/** Ключ стопки с внешним видом категории — всё, что рисует CategoryIcon. */
export type PaymentObjectStackKey = {
  readonly paymentId: string;
  readonly hasOverdue: boolean;
  readonly icon: string;
  readonly color: string;
};

/** Группа стопки карточки: подпись и обрезанный ряд ключей. */
export type PaymentObjectStack = {
  readonly label: string;
  readonly keys: ReadonlyArray<PaymentObjectStackKey>;
};

/** Лимиты иконок стопки: обе группы — по 4, одна — 7 (макет 654:7558,
 * решение владельца 10.09). */
const STACK_LIMIT_BOTH = 4;
const STACK_LIMIT_SINGLE = 7;

/** Стопки карточки объекта: только непустые группы, ключи с иконками
 * категорий фида (нет строки в фиде — дефолт пользовательских), порядок
 * серверный. */
export function paymentObjectStacks(
  object: GlobalPaymentObject,
  paymentsById: ReadonlyMap<string, GlobalPayment>,
): ReadonlyArray<PaymentObjectStack> {
  const single =
    object.autoPayRules.length === 0 || object.otherRules.length === 0;
  const limit = single ? STACK_LIMIT_SINGLE : STACK_LIMIT_BOTH;

  const toStack = (
    label: string,
    rules: GlobalPaymentObject['autoPayRules'],
  ): PaymentObjectStack => ({
    label,
    keys: rules.slice(0, limit).map((rule) => {
      const payment = paymentsById.get(rule.paymentId);
      const style = payment !== undefined
        ? categoryStyle(payment.category.source, payment.category.slug)
        : categoryStyle('custom');
      return {
        paymentId: rule.paymentId,
        hasOverdue: rule.hasOverdue,
        icon: style.icon,
        color: style.color,
      };
    }),
  });

  const stacks: PaymentObjectStack[] = [];
  if (object.autoPayRules.length > 0) {
    stacks.push(toStack('Автоплатежи', object.autoPayRules));
  }
  if (object.otherRules.length > 0) {
    stacks.push(toStack('Платежи', object.otherRules));
  }
  return stacks;
}
