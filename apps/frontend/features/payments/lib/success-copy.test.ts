import { describe, expect, it } from 'vitest';
import { successScreenCopy } from './success-copy';

/** Группировка разрядов в formatMoneyKopecks — неразрывный пробел. */
const RUB = (rubles: string): string => `${rubles.replace(/ /g, '\u00A0')} \u20BD`;

/**
 * Тексты экрана успеха (Figma 835:19893): «Вы создали автоплатеж
 * «Аренда мебели»» + «Платеж пополнится сам 10 сентября на 2 500 ₽, далее…».
 * Хвост «далее …» собирается из recurrenceLabel (резолюция #452) — в макете
 * стоит синонимичная формулировка «каждый месяц того же числа»; каноничный
 * форматтер приложения точнее указывает день.
 */

describe('successScreenCopy', () => {
  it('автоплатёж — формулировка пополнения из фрейма успеха', () => {
    const copy = successScreenCopy({
      draftType: 'autopayment',
      typedTitle: 'Аренда мебели',
      amountKopecks: 250000,
      recurrence: { kind: 'monthly', daysOfMonth: [10], lastDay: false },
      firstOccurrence: '2026-09-10',
    });
    expect(copy.heading).toBe('Вы создали автоплатеж\n«Аренда мебели»');
    expect(copy.description).toBe(
      `Платеж пополнится сам 10 сентября на ${RUB('2 500')}, далее каждый месяц 10 числа`,
    );
  });

  it('обычный платёж — формулировка первого платежа', () => {
    const copy = successScreenCopy({
      draftType: 'payment',
      typedTitle: 'Коммунальные услуги',
      amountKopecks: 480000,
      recurrence: { kind: 'weekly', weekdays: [1] },
      firstOccurrence: '2026-08-31',
    });
    expect(copy.heading).toBe('Вы создали платеж\n«Коммунальные услуги»');
    expect(copy.description).toBe(
      `Первый платеж 31 августа на ${RUB('4 800')}, далее каждый понедельник`,
    );
  });

  it('мультидневная неделя и последний день месяца читаются из канонических меток', () => {
    expect(
      successScreenCopy({
        draftType: 'payment',
        typedTitle: 'Т',
        amountKopecks: 100000,
        recurrence: { kind: 'weekly', weekdays: [0, 3] },
        firstOccurrence: '2026-09-03',
      }).description,
    ).toBe(`Первый платеж 3 сентября на ${RUB('1 000')}, далее каждую неделю в ср, вс`);
    expect(
      successScreenCopy({
        draftType: 'payment',
        typedTitle: 'Т',
        amountKopecks: 100000,
        recurrence: { kind: 'monthly', daysOfMonth: [], lastDay: true },
        firstOccurrence: '2026-08-31',
      }).description,
    ).toBe(`Первый платеж 31 августа на ${RUB('1 000')}, далее последний день каждого месяца`);
  });

  it('название не введено — заголовок одной строкой без «названия»', () => {
    const copy = successScreenCopy({
      draftType: 'payment',
      amountKopecks: 480000,
      recurrence: { kind: 'weekly', weekdays: [1] },
      firstOccurrence: '2026-08-31',
    });
    expect(copy.heading).toBe('Вы создали платеж');
    const auto = successScreenCopy({
      draftType: 'autopayment',
      amountKopecks: 480000,
      recurrence: { kind: 'weekly', weekdays: [1] },
      firstOccurrence: '2026-08-31',
    });
    expect(auto.heading).toBe('Вы создали автоплатеж');
  });

  it('первое вхождение не определено (окончание раньше заведения) — описания нет', () => {
    const copy = successScreenCopy({
      draftType: 'payment',
      typedTitle: 'Страховка',
      amountKopecks: 99000,
      recurrence: { kind: 'daily' },
      firstOccurrence: null,
    });
    expect(copy.heading).toBe('Вы создали платеж\n«Страховка»');
    expect(copy.description).toBeUndefined();
  });
});
