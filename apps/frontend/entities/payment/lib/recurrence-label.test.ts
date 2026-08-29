import { describe, expect, it } from 'vitest';
import type { Recurrence } from '../model/types';
import { recurrenceLabel } from './recurrence-label';

describe('recurrenceLabel — формулировки резолюции #452', () => {
  it('день: «Ежедневно»', () => {
    expect(recurrenceLabel({ kind: 'daily' })).toBe('Ежедневно');
  });

  it.each([
    [1, 'Каждый понедельник'],
    [2, 'Каждый вторник'],
    [4, 'Каждый четверг'],
    [0, 'Каждое воскресенье'],
    [3, 'Каждую среду'],
    [5, 'Каждую пятницу'],
    [6, 'Каждую субботу'],
  ] as const)('один день недели %i → «%s»', (weekday, expected) => {
    expect(recurrenceLabel({ kind: 'weekly', weekdays: [weekday] })).toBe(expected);
  });

  it('несколько дней недели: «Каждую неделю в пн, чт, вс» (порядок от понедельника)', () => {
    expect(
      recurrenceLabel({ kind: 'weekly', weekdays: [0, 1, 4] }),
    ).toBe('Каждую неделю в пн, чт, вс');
  });

  it('порядок коротких имён нормализуется независимо от порядка выбора', () => {
    expect(
      recurrenceLabel({ kind: 'weekly', weekdays: [6, 2] }),
    ).toBe('Каждую неделю в вт, сб');
  });

  it.each([
    [1, 'Каждый месяц 1 числа'],
    [5, 'Каждый месяц 5 числа'],
    [15, 'Каждый месяц 15 числа'],
    [28, 'Каждый месяц 28 числа'],
    [30, 'Каждый месяц 30 числа'],
  ] as const)('месяц, день %i → «%s»', (day, expected) => {
    expect(
      recurrenceLabel({ kind: 'monthly', daysOfMonth: [day], lastDay: false }),
    ).toBe(expected);
  });

  it('несколько дней месяца перечисляются через «и»', () => {
    expect(
      recurrenceLabel({ kind: 'monthly', daysOfMonth: [1, 18], lastDay: false }),
    ).toBe('Каждый месяц 1 и 18 числа');
    expect(
      recurrenceLabel({ kind: 'monthly', daysOfMonth: [1, 5, 18], lastDay: false }),
    ).toBe('Каждый месяц 1, 5 и 18 числа');
  });

  it('дни плюс последний день месяца', () => {
    expect(
      recurrenceLabel({ kind: 'monthly', daysOfMonth: [1, 18], lastDay: true }),
    ).toBe('Каждый месяц 1 и 18 числа и в последний день месяца');
  });

  it('день 31 прижимается в каждом месяце — текст «Последний день каждого месяца»', () => {
    expect(recurrenceLabel({ kind: 'monthly', daysOfMonth: [], lastDay: true })).toBe(
      'Последний день каждого месяца',
    );
  });

  it('год: «Каждое 13 мая»', () => {
    const yearly: Recurrence = { kind: 'yearly', month: 5, day: 13 };
    expect(recurrenceLabel(yearly)).toBe('Каждое 13 мая');
  });

  it('год: первое июня — без лишних нулей и склонённый месяц', () => {
    const yearly: Recurrence = { kind: 'yearly', month: 6, day: 1 };
    expect(recurrenceLabel(yearly)).toBe('Каждое 1 июня');
  });
});
