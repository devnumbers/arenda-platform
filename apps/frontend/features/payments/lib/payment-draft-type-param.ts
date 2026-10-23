import type { PaymentDraftType } from './use-payment-wizard-draft';

/**
 * Тип создания из query-параметра визарда (?type=payment|autopayment):
 * отсутствующее, массивное и неизвестное значение — undefined; страница
 * маршрута сама решает, куда редиректить без типа.
 */
export function parsePaymentDraftTypeParam(value: unknown): PaymentDraftType | undefined {
  return value === 'payment' || value === 'autopayment' ? value : undefined;
}
