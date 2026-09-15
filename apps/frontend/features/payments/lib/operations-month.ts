import type { IsoDate } from '@/entities/payment';

/**
 * Календарный месяц в модели операций: month — 0..11, как у Date.
 * Потребители после дефолта «весь период» (карта #669): целый месяц как
 * шаг листания применённого периода (shiftOperationsPeriod) и месячная
 * сводка секции «Операции в <месяц>» на странице объекта (#473 —
 * сознательно осталась месячной). «Сегодня» — клиентское (та же
 * оговорка про TZ: зоны смотрящего сервер не сообщает, расхождение с TZ
 * собственника ограничено краевыми часами).
 */
export type OperationsMonth = {
  readonly year: number;
  readonly month: number;
};

/** Месяц, в который попадает ISO-дата «сегодня». */
export function operationsMonthOf(today: IsoDate): OperationsMonth {
  return { year: Number(today.slice(0, 4)), month: Number(today.slice(5, 7)) - 1 };
}

/** Непрерывный индекс месяца (год × 12 + месяц) — для сравнения «раньше/позже». */
export function operationsMonthIndex(month: OperationsMonth): number {
  return month.year * 12 + month.month;
}

/** Сдвиг на целое число месяцев через границы года (стрелки навигации). */
export function shiftOperationsMonth(base: OperationsMonth, delta: number): OperationsMonth {
  const months = operationsMonthIndex(base) + delta;
  return { year: Math.floor(months / 12), month: months % 12 };
}

/** Включительные границы месяца 'YYYY-MM-DD'. */
export function operationsMonthRange(month: OperationsMonth): {
  from: IsoDate;
  to: IsoDate;
} {
  const lastDay = new Date(month.year, month.month + 1, 0).getDate();
  const prefix = `${month.year}-${String(month.month + 1).padStart(2, '0')}`;
  return {
    from: `${prefix}-01`,
    to: `${prefix}-${String(lastDay).padStart(2, '0')}`,
  };
}
