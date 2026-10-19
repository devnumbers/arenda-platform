import { describe, expect, it } from 'vitest';
import { auditActionChoices } from '../fields';

// Реестр действий аудита целиком — apps/backend/internal/audit/domain/entry.go
// (значения Action). Каталог админки — осознанное подмножество реестра: админке
// не нужны все действия доменов, поэтому контракт синхронизации — «каталог не
// ссылается на то, чего в реестре нет»: переименование или удаление действия
// на бэке оставило бы в каталоге мёртвый id и падает здесь, а не чипом с сырым
// id в журнале. После карты #1525 (email-only, ADR 0067) телефонных действий
// в реестре нет: auth.phone_changed заменён парой
// auth.email_changed/auth.email_change_failed (протокол #721 сохранён,
// телефонный протокол удалён #1528).
const backendRegistryActions: ReadonlySet<string> = new Set([
  'auth.email_change_failed',
  'auth.email_changed',
  'auth.login',
  'auth.login_failed',
  'auth.logout',
  'auth.other_sessions_revoked',
  'auth.registered',
  'auth.session_revoked',
  'contact.created',
  'contact.deleted',
  'contact.photo_added',
  'contact.photo_deleted',
  'contact.updated',
  'lease.completed',
  'lease.created',
  'lease.updated',
  'operation.completed',
  'operation.created',
  'operation.deleted',
  'operation.marked_incomplete',
  'operation.moved',
  'operation.paid',
  'operation.updated',
  'operation_category.created',
  'payment.completed',
  'payment.created',
  'payment.deleted',
  'payment.paused',
  'payment.resumed',
  'payment.updated',
  'payment_method.activated',
  'payment_method.added',
  'payment_method.deleted',
  'profile.photo_added',
  'profile.photo_deleted',
  'profile.updated',
  'property.archived',
  'property.created',
  'property.deleted',
  'property.photo_added',
  'property.photo_deleted',
  'property.unarchived',
  'property.updated',
  'property_contact.created',
  'property_contact.deleted',
  'property_contact.updated',
  'property_member.added',
  'property_member.left',
  'property_member.reactivated',
  'property_member.removed',
  'property_member.suspended',
  'property_member.updated',
  'property_member_invitation.activated',
  'property_member_invitation.cancelled',
  'property_member_invitation.invited',
  'property_member_invitation.resent',
  'property_member_invitation.role_changed',
  'recurring_operation.created',
  'recurring_operation.deleted',
  'recurring_operation.paused',
  'recurring_operation.resumed',
  'recurring_operation.updated',
  'rental.completed',
  'rental.created',
  'rental.deleted',
  'rental.updated',
  'subscription.auto_renew_toggled',
  'subscription.cancelled',
  'subscription.grace_extended',
  'subscription.resumed',
  'subscription.service_assigned',
  'subscription.tariff_changed',
  'subscription.tariff_forced',
  'subscription.time_shifted',
  'subscription_payment.failed',
  'subscription_payment.refunded',
  'subscription_payment.succeeded',
  'subscription_payment.synced',
  'tariff.created',
  'tariff.updated',
  'task.completed',
  'task.completed_cleared',
  'task.uncompleted',
  'task_rule.created',
  'task_rule.deleted',
  'task_rule.updated',
  'tenant_contact.created',
  'tenant_contact.updated',
]);

// Пара действий смены email: единственная живая замена снесённого
// auth.phone_changed — должна быть подписана в админке всегда.
const emailChangeActions = ['auth.email_changed', 'auth.email_change_failed'] as const;

// Отставленные действия: продюсер на бэке снесён, но исторические записи
// журнала продолжают нести строку — подпись держится, чтобы старые строки
// читались (прецедент 4c3b43571: «строка auth.logout_all в админке остаётся»).
const retiredActions: ReadonlySet<string> = new Set([
  'auth.logout_all', // POST /auth/logout-all снесён #754, записи остались
  'notification_preferences.updated', // per-channel настройки снесены #738, записи остались
]);

describe('auditActionChoices', () => {
  it('references only registry actions or retired ones with historical records', () => {
    const known = (id: string): boolean => backendRegistryActions.has(id) || retiredActions.has(id);
    const orphans = auditActionChoices.filter((choice) => !known(choice.id));
    expect(orphans).toEqual([]);
  });

  it('contains no phone actions (map #1525, ADR 0067)', () => {
    const phoneChoices = auditActionChoices.filter((choice) => choice.id.includes('phone'));
    expect(phoneChoices).toEqual([]);
  });

  it.each(emailChangeActions)('covers %s exactly once', (action) => {
    const matches = auditActionChoices.filter((choice) => choice.id === action);
    expect(matches).toHaveLength(1);
  });

  it.each(emailChangeActions)('labels %s with a non-empty Russian name', (action) => {
    const choice = auditActionChoices.find((item) => item.id === action);
    expect(choice?.name.trim() ?? '').not.toBe('');
  });

  it('has no duplicate ids', () => {
    const ids = auditActionChoices.map((choice) => choice.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('every choice has a non-empty Russian label', () => {
    for (const choice of auditActionChoices) {
      expect(choice.name.trim()).not.toBe('');
    }
  });
});
