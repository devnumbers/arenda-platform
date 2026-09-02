import type { IsoDate } from '@/entities/payment';
import { MONTH_LABELS } from '@/shared/ui/design/month-grid';

/**
 * Месяц навигации экранов «Доходы объекта» / «Расходы объекта» (#475):
 * month — 0..11, как у Date. Листание стрелками меняет этот месяц,
 * чип периода и H1-сумма следуют за ним; «сегодня» — клиентское (та же
 * оговорка, что на главном экране операций: зоны смотрящего сервер не
 * сообщает, расхождение с TZ собственника ограничено краевыми часами).
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

/** ISO-дата дня месяца 'YYYY-MM-DD'. */
export function operationsMonthIso(month: OperationsMonth, day: number): IsoDate {
  return `${month.year}-${String(month.month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

/** Сдвиг на целое число месяцев через границы года (стрелки навигации). */
export function shiftOperationsMonth(base: OperationsMonth, delta: number): OperationsMonth {
  const months = operationsMonthIndex(base) + delta;
  return { year: Math.floor(months / 12), month: months % 12 };
}

/** Включительные границы месяца 'YYYY-MM-DD' и подпись чипа «Сентябрь 2026». */
export function operationsMonthRange(month: OperationsMonth): {
  from: IsoDate;
  to: IsoDate;
  label: string;
} {
  const lastDay = new Date(month.year, month.month + 1, 0).getDate();
  const prefix = `${month.year}-${String(month.month + 1).padStart(2, '0')}`;
  return {
    from: `${prefix}-01`,
    to: `${prefix}-${String(lastDay).padStart(2, '0')}`,
    label: `${MONTH_LABELS[month.month] ?? ''} ${month.year}`.trim(),
  };
}

