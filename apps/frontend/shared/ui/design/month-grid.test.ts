import { describe, expect, it } from 'vitest';
import {
  MONTH_LABELS,
  WEEKDAY_LABELS,
  daysInMonth,
  firstWeekdayOfMonth,
  isCalendarDay,
  monthTitle,
} from './month-grid';

describe('month-grid', () => {
  it('названия месяцев — 12 в именительном падеже', () => {
    expect(MONTH_LABELS).toHaveLength(12);
    expect(MONTH_LABELS[7]).toBe('Август');
  });

  it('неделя начинается с понедельника', () => {
    expect(WEEKDAY_LABELS).toEqual(['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']);
  });

  it('заголовок месяца: «Август, 2026»', () => {
    expect(monthTitle(2026, 7)).toBe('Август, 2026');
    expect(monthTitle(2024, 0)).toBe('Январь, 2024');
  });

  it('дни в месяце: 31/30/28 и високосный февраль', () => {
    expect(daysInMonth(2026, 7)).toBe(31); // август
    expect(daysInMonth(2026, 8)).toBe(30); // сентябрь
    expect(daysInMonth(2026, 1)).toBe(28); // обычный февраль
    expect(daysInMonth(2024, 1)).toBe(29); // високосный февраль
  });

  it('первый день месяца: Пн = 0 … Вс = 6', () => {
    // 1 августа 2026 — суббота (в макете «1» в 6-м столбце, Figma 835:20041)
    expect(firstWeekdayOfMonth(2026, 7)).toBe(5);
    // 1 сентября 2026 — вторник
    expect(firstWeekdayOfMonth(2026, 8)).toBe(1);
    // 1 февраля 2026 — воскресенье
    expect(firstWeekdayOfMonth(2026, 1)).toBe(6);
  });

  it('сравнение даты без времени', () => {
    expect(isCalendarDay(new Date(2026, 7, 17, 23, 59), 2026, 7, 17)).toBe(true);
    expect(isCalendarDay(new Date(2026, 7, 17), 2026, 8, 17)).toBe(false);
    expect(isCalendarDay(new Date(2026, 7, 17), 2026, 7, 18)).toBe(false);
  });
});
