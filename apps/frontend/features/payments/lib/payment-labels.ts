import type { PaymentForm, PaymentType } from '@/entities/payment';

/**
 * Подписи признаков правила (Figma 834:19662/705:10034): направление
 * («Доход»/«Расход») и форма оплаты («Перевод»/«Наличные»). Общий источник
 * для экрана правки, шага суммы визарда и карточки платежа — одно значение
 * пишется на чипе-переключателе, строке-переключателе и карточке.
 */
export const TYPE_LABELS: Record<PaymentType, string> = {
  income: 'Доход',
  expense: 'Расход',
};

export const FORM_OF_PAYMENT_LABELS: Record<PaymentForm, string> = {
  transfer: 'Перевод',
  cash: 'Наличные',
};
