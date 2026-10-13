import { describe, expect, it } from 'vitest';
import { notificationCircleVisual } from './circle-visual';
import type { NotificationPaymentCategoryIcon } from './circle-visual';

/** Приоритет кругляша платёжных уведомлений — решение владельца 13.10
 * (#1348): иконка категории платежа главнее фото объекта; деградация для
 * строк без снимка — фото, затем стандартная иконка категории уведомлений. */
describe('notificationCircleVisual — приоритет кругляша (#1348)', () => {
  const icon: NotificationPaymentCategoryIcon = { component: () => null, color: '#2B7FFF' };

  it('снимок категории главнее фото объекта', () => {
    const circle = notificationCircleVisual(icon, '/api/properties/x/photo');
    expect(circle).toStrictEqual({ kind: 'paymentCategory', icon });
  });

  it('снимок без фото — снимок', () => {
    expect(notificationCircleVisual(icon, undefined)).toStrictEqual({ kind: 'paymentCategory', icon });
  });

  it('без снимка — фото объекта (старые строки ленты)', () => {
    expect(notificationCircleVisual(undefined, '/api/properties/x/photo')).toStrictEqual({ kind: 'photo' });
  });

  it('без снимка и без фото — стандартная иконка категории', () => {
    expect(notificationCircleVisual(undefined, undefined)).toStrictEqual({ kind: 'standard' });
  });
});
