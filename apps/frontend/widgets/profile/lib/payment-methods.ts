import type { PaymentMethod } from '@/entities/billing';
import { cardNumberTail } from './tariff-about';

/** Названия платёжных систем для строк способов оплаты (#625, макет
 * 1879-71079: «Мир •••• 0700») — из контракта #614 (cardSystem по
 * BIN-префиксу). Название банка-эмитента провайдер не отдаёт и в
 * интерфейсе не называется (решение владельца 11.09, #614). Без
 * распознанной системы — только хвост маски, как в «Операциях» (#624). */
const CARD_SYSTEM_LABELS: Record<PaymentMethod['cardSystem'], string | undefined> = {
  mir: 'Мир',
  visa: 'Visa',
  mastercard: 'Mastercard',
  unknown: undefined,
};

/** Заголовок строки способа оплаты: «Мир •••• 0700» (#625, макет
 * 1879-71079). Хвост маски — общий cardNumberTail (#622-формат). */
export function paymentMethodTitle(
  method: Pick<PaymentMethod, 'displayMask' | 'cardSystem'>,
): string {
  const label = CARD_SYSTEM_LABELS[method.cardSystem];
  const tail = cardNumberTail(method.displayMask);
  return label === undefined ? tail : `${label} ${tail}`;
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
