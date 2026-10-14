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
    expect(notificationActionView('rental_completed', 'rental_extend', fullPayload)).toStrictEqual({
      label: 'Продлить',
      href: `/properties/${PROPERTY_ID}/rentals/extend`,
      variant: 'secondary',
    });
    expect(notificationActionView('rental_completed', 'rental_complete', fullPayload)).toStrictEqual({
      label: 'Завершить',
      href: `/properties/${PROPERTY_ID}/rentals/complete`,
      variant: 'primary',
    });
  });

  it('платёж и задача ведут на свои экраны', () => {
    expect(notificationActionView('payment_due', 'open_payment', fullPayload)).toStrictEqual({
      label: 'Оплатить',
      href: `/properties/${PROPERTY_ID}/payments/${PAYMENT_ID}`,
      variant: 'primary',
    });
    expect(notificationActionView('task_overdue', 'open_task', fullPayload)).toStrictEqual({
      label: 'Выполнить',
      href: `/properties/${PROPERTY_ID}/tasks/${TASK_RULE_ID}/edit`,
      variant: 'primary',
    });
    // Экран задачи — экран её правила (#750): без объекта — плоский
    // маршрут (ADR 0052).
    expect(
      notificationActionView('task_overdue', 'open_task', { taskId: TASK_ID, taskRuleId: TASK_RULE_ID }),
    ).toStrictEqual({
      label: 'Выполнить',
      href: `/tasks/${TASK_RULE_ID}/edit`,
      variant: 'primary',
    });
  });

  it('open_property: лейбл по паре (event_type, action) — решение владельца 12.10.2026, #1359', () => {
    // Приглашение: «В объект» вместо «Принять» — приглашение принимается
    // по email-ссылке, лента лишь навигирует.
    expect(notificationActionView('property_invitation', 'open_property', fullPayload)).toStrictEqual({
      label: 'В объект',
      href: `/properties/${PROPERTY_ID}`,
      variant: 'primary',
    });
    // Смена роли: «Перейти в объект» — «Участники» (owner-only /participants)
    // получателю-кнопкой больше не светится.
    expect(notificationActionView('access_role_changed', 'open_property', fullPayload)).toStrictEqual({
      label: 'Перейти в объект',
      href: `/properties/${PROPERTY_ID}`,
      variant: 'primary',
    });
    // Принятому приглашению (владельцу) — по-прежнему «Участники».
    expect(notificationActionView('invitation_accepted', 'open_property_members', fullPayload)).toStrictEqual({
      label: 'Участники',
      href: '/participants',
      variant: 'primary',
    });
  });

  it('глобальные экраны не требуют payload-ссылок', () => {
    // №14 «Тариф изменён»: лейбл не из макета (у №14 его нет), а из
    // каталога #737 (#788) — обоснование в notification-actions.ts.
    expect(notificationActionView('subscription_plan_changed', 'open_tariffs', fullPayload)).toStrictEqual({
      label: 'Тариф',
      href: '/profile/tariff',
      variant: 'primary',
    });
    expect(notificationActionView('subscription_grace_entered', 'open_payment_methods', fullPayload)?.href).toBe(
      '/profile/tariff/payment-methods',
    );
  });

  it('кнопка без ссылки не рендерится — переход строится только из живых ссылок payload', () => {
    expect(notificationActionView('rental_completed', 'rental_extend', {})).toBeNull();
    expect(
      notificationActionView('rental_completed', 'rental_complete', { property: { id: PROPERTY_ID, name: 'x' } }),
    ).not.toBeNull();
    expect(notificationActionView('payment_due', 'open_payment', { paymentId: PAYMENT_ID })).toBeNull();
    expect(notificationActionView('payment_due', 'open_payment', fullPayload)).not.toBeNull();
    expect(notificationActionView('task_overdue', 'open_task', {})).toBeNull();
    expect(notificationActionView('property_invitation', 'open_property', {})).toBeNull();
  });
});

describe('полный каталог действий #737 покрыт', () => {
  it.each(NOTIFICATION_ACTION_KINDS)('%s даёт кнопку с лейблом', (kind) => {
    const view = notificationActionView('property_invitation', kind, fullPayload);
    expect(view).not.toBeNull();
    expect(view?.label.length).toBeGreaterThan(0);
  });
});
