import { describe, expect, it } from 'vitest';
import { notificationHasWarningBadge } from './event-badge';

describe('notificationHasWarningBadge', () => {
  it('ставит бейдж только на тарифных событиях «что-то не так» (#1164)', () => {
    expect(notificationHasWarningBadge('subscription_payment_failed')).toBe(true);
    expect(notificationHasWarningBadge('subscription_grace_entered')).toBe(true);
    expect(notificationHasWarningBadge('subscription_grace_expiring')).toBe(true);
  });

  it('не ставит бейдж на тарифных событиях без проблемы', () => {
    expect(notificationHasWarningBadge('subscription_payment_succeeded')).toBe(false);
    expect(notificationHasWarningBadge('subscription_plan_changed')).toBe(false);
    expect(notificationHasWarningBadge('subscription_payment_reminder')).toBe(false);
  });

  it('не ставит бейдж на событиях других категорий каталога v1', () => {
    expect(notificationHasWarningBadge('payment_due')).toBe(false);
    expect(notificationHasWarningBadge('payment_overdue')).toBe(false);
    expect(notificationHasWarningBadge('payment_reminder')).toBe(false);
    expect(notificationHasWarningBadge('task_overdue')).toBe(false);
    expect(notificationHasWarningBadge('rental_completed')).toBe(false);
    expect(notificationHasWarningBadge('property_invitation')).toBe(false);
    expect(notificationHasWarningBadge('system_maintenance')).toBe(false);
  });

  it('не ставит бейдж по умолчанию — неизвестный или пустой тип события', () => {
    expect(notificationHasWarningBadge('subscription_auto_payment_executed')).toBe(false);
    expect(notificationHasWarningBadge('')).toBe(false);
  });
});
