import type { PaymentMethod } from '@/entities/billing';
import { cardNumberTail } from './tariff-about';

/** Заголовок строки способа оплаты: только хвост маски «•••• 0700» —
 * бренд платёжной системы (Мир/Visa/Mastercard) в интерфейсе не
 * выводим, подпись системы из макета 1879-71079 убрана решением
 * владельца 15.09 (#611); тот же формат, что на «О тарифе» (#622) и в
 * «Операциях» (#624). Название банка-эмитента провайдер не отдаёт и в
 * интерфейсе не называется (решение владельца 11.09, #614). Хвост —
 * общий cardNumberTail (#622-формат). */
export function paymentMethodTitle(
  method: Pick<PaymentMethod, 'displayMask'>,
): string {
  return cardNumberTail(method.displayMask);
}

/** Какой гард открыть по корзине: активную карту бэк не удаляет
 * (409 in use), единственная карта всегда активна — тексты разные
 * (#625, макеты 1936-114196 активная / 1936-113689 единственная).
 * Неактивная карта удаляется через подтверждение — гард не нужен. */
export function paymentMethodDeleteGuard(
  method: Pick<PaymentMethod, 'isActive'>,
  totalCount: number,
): 'active' | 'only' | undefined {
  if (!method.isActive) {
    return undefined;
  }
  return totalCount <= 1 ? 'only' : 'active';
}
