import type { Payment } from '@/entities/payment';

/**
 * Состав круглых действий экрана платежа (#818): «Оплатить» остаётся всегда —
 * канон отметки оплаты месяца аренды; «Изменить» и пауза скрываются у
 * управляемого платежа (isRentalManaged — платёж аренды правится только
 * через аренду, мутации правила дают 409), вместо «Изменить» у
 * незавершённой аренды живёт «Изменить аренду» (#1158): условия правятся
 * на экране аренды. Завершённое правило живёт без паузы (история 44 спеки
 * #453) — по клиентскому предикату isPaymentCompleted: серверный
 * isCompleted у бессрочной паузы уже «завершен» (ничего больше не
 * материализует), а пауза на экране остаётся возобновляемой. У управляемого
 * платежа решает состояние аренды (isRentalCompleted), не предикат правила:
 * «Ожидает действия» держит isCompleted-истиной при живой правке условий.
 * Чистая функция над сущностью — витест держит матрицу, живая приёмка —
 * проводку.
 */
export type PaymentDetailAction = 'pause' | 'resume' | 'edit' | 'editRental' | 'pay';

export function paymentDetailActions(
  payment: Pick<Payment, 'isRentalManaged' | 'isRentalCompleted'>,
  paused: boolean,
  completed: boolean,
): ReadonlyArray<PaymentDetailAction> {
  if (payment.isRentalManaged) {
    return payment.isRentalCompleted ? ['pay'] : ['editRental', 'pay'];
  }
  if (completed) {
    return ['edit', 'pay'];
  }
  return paused ? ['resume', 'edit', 'pay'] : ['pause', 'edit', 'pay'];
}
