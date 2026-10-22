import { describe, expect, it } from 'vitest';
import { emailNotificationsRowDisabled } from './email-notifications-row';

/** Гейт ряда «Уведомления на почту» (решение владельца 22.10.2026):
 * пер-платёжный выбор «не надо» гасит весь ряд — уведомлений и
 * напоминаний по платежу нет, шоткату глобальной email-настройки
 * нечего включать. */
describe('emailNotificationsRowDisabled', () => {
  it('ручной платёж: «Не напоминать» гасит ряд', () => {
    expect(
      emailNotificationsRowDisabled('payment', {
        reminderOffsetDays: undefined,
        notifyAutoPaid: undefined,
      }),
    ).toBe(true);
  });

  it('ручной платёж: выбранное напоминание держит ряд живым', () => {
    for (const reminderOffsetDays of [1, 3, 7] as const) {
      expect(
        emailNotificationsRowDisabled('payment', {
          reminderOffsetDays,
          notifyAutoPaid: undefined,
        }),
      ).toBe(false);
    }
  });

  it('автоплатёж: дефолт «Не уведомлять» (undefined) гасит ряд', () => {
    expect(
      emailNotificationsRowDisabled('autopayment', {
        reminderOffsetDays: undefined,
        notifyAutoPaid: undefined,
      }),
    ).toBe(true);
  });

  it('автоплатёж: явное «Не уведомлять» (false, состояние правки) гасит ряд', () => {
    expect(
      emailNotificationsRowDisabled('autopayment', {
        reminderOffsetDays: undefined,
        notifyAutoPaid: false,
      }),
    ).toBe(true);
  });

  it('автоплатёж: «Да, уведомлять» держит ряд живым', () => {
    expect(
      emailNotificationsRowDisabled('autopayment', {
        reminderOffsetDays: undefined,
        notifyAutoPaid: true,
      }),
    ).toBe(false);
  });
});
