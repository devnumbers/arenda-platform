import type { FC, SVGProps } from 'react';

/**
 * Разрешённый кругляш категории платежа (#1348): компонент bold-иконки
 * каталога (рисуется белым, размер задаёт потребитель по своему варианту)
 * и цвет подложки каталога. Структура — PaymentCategoryVisual из
 * features/payment-categories; собственный тип — слой entities не
 * импортирует features, потребитель скармливает структурно совместимый.
 */
export type NotificationPaymentCategoryIcon = Readonly<{
  component: FC<SVGProps<SVGSVGElement>>;
  color: string;
}>;

/**
 * Кругляш строки уведомления — приоритет решения владельца 13.10 (#1348):
 * снимок категории платежа рисуется всегда, когда есть (платёжное событие —
 * про категорию платежа), фото объекта отступает; деградация для строк без
 * снимка (старая лента, другие категории) — фото (#1275), затем
 * стандартная иконка категории уведомлений.
 */
export function notificationCircleVisual(
  paymentCategoryIcon: NotificationPaymentCategoryIcon | undefined,
  photoUrl: string | undefined,
): { kind: 'paymentCategory'; icon: NotificationPaymentCategoryIcon } | { kind: 'photo' } | { kind: 'standard' } {
  if (paymentCategoryIcon !== undefined) return { kind: 'paymentCategory', icon: paymentCategoryIcon };
  if (photoUrl !== undefined) return { kind: 'photo' };
  return { kind: 'standard' };
}
