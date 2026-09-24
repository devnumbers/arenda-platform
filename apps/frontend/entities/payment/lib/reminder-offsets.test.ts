import { describe, expect, it } from 'vitest';
import { PAYMENT_REMINDER_OPTIONS, paymentReminderOptionLabel } from './reminder-offsets';

describe('опции напоминания о платеже (карта #822)', () => {
  it('контрактные оффсеты 1/3/7 в порядке макета с метками нужного падежа', () => {
    expect(PAYMENT_REMINDER_OPTIONS.map((option) => option.offset)).toStrictEqual([1, 3, 7]);
    expect(paymentReminderOptionLabel(1)).toBe('За 1 день');
    expect(paymentReminderOptionLabel(3)).toBe('За 3 дня');
    expect(paymentReminderOptionLabel(7)).toBe('За 7 дней');
  });

  it('у каждой опции кортежа есть метка — пикер не рисует пустых строк', () => {
    for (const option of PAYMENT_REMINDER_OPTIONS) {
      expect(option.label.length).toBeGreaterThan(0);
      expect(paymentReminderOptionLabel(option.offset)).toBe(option.label);
    }
  });
});
