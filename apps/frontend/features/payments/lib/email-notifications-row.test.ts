import { describe, expect, it } from 'vitest';
import {
  emailNotificationsRowDisabled,
  emailNotificationsRowState,
  emailNotificationsRowValue,
} from './email-notifications-row';

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

/** Действующее значение ряда в двухуровневой модели (карта #1551, канон
 * #1552, решение владельца 22.10.2026 «надо запоминать»): свитч —
 * персональная галочка почты платежа; гейт («не надо») показывает и
 * отправляет off, но запомненное значение формы не стирает — возврат
 * радио в «да» возвращает его. */
describe('emailNotificationsRowValue', () => {
  it('гейт («не надо») показывает и отправляет off при любом запомненном', () => {
    expect(emailNotificationsRowValue(true, true)).toBe(false);
    expect(emailNotificationsRowValue(true, false)).toBe(false);
  });

  it('живой ряд показывает запомненное значение формы', () => {
    expect(emailNotificationsRowValue(false, true)).toBe(true);
    expect(emailNotificationsRowValue(false, false)).toBe(false);
  });
});

/** Композит (гейт + значение) — форма, в которой ряд потребляют экраны
 * и модели команд (карта #1551). */
describe('emailNotificationsRowState', () => {
  it('гейт отдаёт пару disabled + off при любом запомненном', () => {
    expect(
      emailNotificationsRowState('payment', { reminderOffsetDays: undefined, notifyAutoPaid: undefined }, true),
    ).toStrictEqual({ disabled: true, value: false });
    expect(
      emailNotificationsRowState('autopayment', { reminderOffsetDays: undefined, notifyAutoPaid: undefined }, false),
    ).toStrictEqual({ disabled: true, value: false });
  });

  it('живый ряд отдаёт disabled: false и запомненное значение', () => {
    expect(
      emailNotificationsRowState('payment', { reminderOffsetDays: 1, notifyAutoPaid: undefined }, false),
    ).toStrictEqual({ disabled: false, value: false });
    expect(
      emailNotificationsRowState('autopayment', { reminderOffsetDays: undefined, notifyAutoPaid: true }, true),
    ).toStrictEqual({ disabled: false, value: true });
  });
});
