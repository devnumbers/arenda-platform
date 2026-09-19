import { describe, expect, it } from 'vitest';
import { notificationActionView } from './notification-actions';
import {
  NOTIFICATION_ACTION_KINDS,
  type NotificationPayload,
} from '@/entities/notification';

const PROPERTY_ID = '0194a3f8-0000-7000-8000-000000000001';
const PAYMENT_ID = '0194a3f8-0000-7000-8000-000000000002';
const TASK_ID = '0194a3f8-0000-7000-8000-000000000003';
const TASK_RULE_ID = '0194a3f8-0000-7000-8000-000000000004';

const fullPayload: NotificationPayload = {
  property: { id: PROPERTY_ID, name: '2-комнатная на Ленина' },
  paymentId: PAYMENT_ID,
  taskId: TASK_ID,
  taskRuleId: TASK_RULE_ID,
};

describe('notificationActionView', () => {
  it('пара аренды: Продлить (secondary) и Завершить (primary) на визарды аренды объекта', () => {
    expect(notificationActionView('rental_extend', fullPayload)).toStrictEqual({
      label: 'Продлить',
      href: `/properties/${PROPERTY_ID}/rentals/extend`,
      variant: 'secondary',
    });
    expect(notificationActionView('rental_complete', fullPayload)).toStrictEqual({
      label: 'Завершить',
      href: `/properties/${PROPERTY_ID}/rentals/complete`,
      variant: 'primary',
    });
  });

  it('платёж и задача ведут на свои экраны', () => {
    expect(notificationActionView('open_payment', fullPayload)).toStrictEqual({
      label: 'Оплатить',
      href: `/properties/${PROPERTY_ID}/payments/${PAYMENT_ID}`,
      variant: 'primary',
    });
    expect(notificationActionView('open_task', fullPayload)).toStrictEqual({
      label: 'Выполнить',
      href: `/properties/${PROPERTY_ID}/tasks/${TASK_RULE_ID}/edit`,
      variant: 'primary',
    });
    // Экран задачи — экран её правила (#750): без объекта — плоский
    // маршрут (ADR 0052).
    expect(
      notificationActionView('open_task', { taskId: TASK_ID, taskRuleId: TASK_RULE_ID }),
    ).toStrictEqual({
      label: 'Выполнить',
      href: `/tasks/${TASK_RULE_ID}/edit`,
      variant: 'primary',
    });
  });

  it('глобальные экраны не требуют payload-ссылок', () => {
    expect(notificationActionView('open_property', fullPayload)?.href).toBe(`/properties/${PROPERTY_ID}`);
    expect(notificationActionView('open_property_members', fullPayload)?.href).toBe('/participants');
    expect(notificationActionView('open_tariffs', fullPayload)?.href).toBe('/profile/tariff');
    expect(notificationActionView('open_payment_methods', fullPayload)?.href).toBe(
      '/profile/tariff/payment-methods',
    );
  });

  it('кнопка без ссылки не рендерится — переход строится только из живых ссылок payload', () => {
    expect(notificationActionView('rental_extend', {})).toBeNull();
    expect(notificationActionView('rental_complete', { property: { id: PROPERTY_ID, name: 'x' } })).not.toBeNull();
    expect(notificationActionView('open_payment', { paymentId: PAYMENT_ID })).toBeNull();
    expect(notificationActionView('open_payment', fullPayload)).not.toBeNull();
    expect(notificationActionView('open_task', {})).toBeNull();
    expect(notificationActionView('open_property', {})).toBeNull();
  });
});

describe('полный каталог действий #737 покрыт', () => {
  it.each(NOTIFICATION_ACTION_KINDS)('%s даёт кнопку с лейблом', (kind) => {
    const view = notificationActionView(kind, fullPayload);
    expect(view).not.toBeNull();
    expect(view?.label.length).toBeGreaterThan(0);
  });
});
