import { describe, expect, it } from 'vitest';
import { successScreenCopy } from './success-copy';

/**
 * Тексты экрана успеха (Figma 835:19893, структура — канон успеха операций
 * 1858:105544, решение владельца 01.10): «Вы создали автоплатеж
 * «Аренда мебели»» + «Платеж пополнится сам 10 сентября, далее…». Сумма на
 * экране — большой блок со знаком (рисует компонент), в описании её нет.
 * Хвост «далее …» собирается из recurrenceLabel (резолюция #452).
 */

describe('successScreenCopy', () => {
  it('автоплатёж — формулировка пополнения из фрейма успеха, без суммы в описании', () => {
    const copy = successScreenCopy({
      draftType: 'autopayment',
      typedTitle: 'Аренда мебели',
      recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
      firstOccurrence: '2026-09-10',
    });
    expect(copy.heading).toBe('Вы создали автоплатеж\n«Аренда мебели»');
    expect(copy.description).toBe('Платеж пополнится сам 10 сентября, далее каждый месяц 10 числа');
  });

  it('обычный платёж — формулировка первого платежа', () => {
    const copy = successScreenCopy({
      draftType: 'payment',
      typedTitle: 'Коммунальные услуги',
      recurrence: { kind: 'weekly', weekdays: [1] },
      firstOccurrence: '2026-08-31',
    });
    expect(copy.heading).toBe('Вы создали платеж\n«Коммунальные услуги»');
    expect(copy.description).toBe('Первый платеж 31 августа, далее каждый понедельник');
  });

  it('мультидневная неделя и последний день месяца читаются из канонических меток', () => {
    expect(
      successScreenCopy({
        draftType: 'payment',
        typedTitle: 'Т',
        recurrence: { kind: 'weekly', weekdays: [0, 3] },
        firstOccurrence: '2026-09-03',
      }).description,
    ).toBe('Первый платеж 3 сентября, далее каждую неделю в ср, вс');
    expect(
      successScreenCopy({
        draftType: 'payment',
        typedTitle: 'Т',
        recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
        firstOccurrence: '2026-08-31',
      }).description,
    ).toBe('Первый платеж 31 августа, далее последний день каждого месяца');
  });

  it('название не введено — заголовок одной строкой без «названия»', () => {
    const copy = successScreenCopy({
      draftType: 'payment',
      recurrence: { kind: 'weekly', weekdays: [1] },
      firstOccurrence: '2026-08-31',
    });
    expect(copy.heading).toBe('Вы создали платеж');
    const auto = successScreenCopy({
      draftType: 'autopayment',
      recurrence: { kind: 'weekly', weekdays: [1] },
      firstOccurrence: '2026-08-31',
    });
    expect(auto.heading).toBe('Вы создали автоплатеж');
  });

  it('первое вхождение не определено (окончание раньше заведения) — описания нет', () => {
    const copy = successScreenCopy({
      draftType: 'payment',
      typedTitle: 'Страховка',
      recurrence: { kind: 'daily' },
      firstOccurrence: null,
    });
    expect(copy.heading).toBe('Вы создали платеж\n«Страховка»');
    expect(copy.description).toBeUndefined();
  });
});
