import { describe, expect, it } from 'vitest';
import {
  addDays,
  formatDateShort,
  formatDateWithWeekday,
  weekDates,
  weekdayShort,
} from './dates';

/**
 * Пин недельного инварианта дейт-хелперов: weekDates возвращает ровно семь
 * дат пн–вс (тип — 7-кортеж с волны B noUncheckedIndexedAccess), а форматтеры
 * реально находят значения в таблицах дней/месяцев (их лукапы получили явные
 * `?? ''`-дефолты — пустая строка в снапшоте сразу показала бы сломанный
 * индекс).
 */
describe('weekDates', () => {
  it('возвращает семь дат пн–вс недели якоря', () => {
    // 2026-08-22 — суббота; неделя начинается в понедельник 2026-08-17.
    expect(weekDates('2026-08-22')).toStrictEqual([
      '2026-08-17',
      '2026-08-18',
      '2026-08-19',
      '2026-08-20',
      '2026-08-21',
      '2026-08-22',
      '2026-08-23',
    ]);
  });

  it('начало недели — понедельник, включая переход года', () => {
    // 2027-01-01 — пятница; её неделя начинается в понедельник 2026-12-28.
    expect(weekDates('2027-01-01')[0]).toBe('2026-12-28');
    // Якорь-понедельник остаётся началом своей недели.
    expect(weekDates('2026-08-17')[0]).toBe('2026-08-17');
  });

  it('все семь дат — последовательные дни от понедельника', () => {
    const days = weekDates('2026-10-14');
    const monday = days[0];
    if (monday === undefined) {
      throw new Error('weekDates must return seven dates');
    }
    for (let i = 1; i < 7; i += 1) {
      expect(days[i]).toBe(addDays(monday, i));
    }
  });
});

describe('табличные лукапы форматтеров', () => {
  it('weekdayShort даёт короткое имя дня', () => {
    expect(weekdayShort('2026-08-22')).toBe('Сб');
    expect(weekdayShort('2026-08-17')).toBe('Пн');
  });

  it('formatDateWithWeekday даёт «число месяц, день недели»', () => {
    expect(formatDateWithWeekday('2026-08-22')).toBe('22 августа, суббота');
  });

  it('formatDateShort даёт «день месяц, день»', () => {
    expect(formatDateShort('2026-08-22')).toBe('22 авг, Сб');
  });
});
