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

/** Сдвиг на целое число месяцев через границы года (стрелки навигации). */
export function shiftOperationsMonth(base: OperationsMonth, delta: number): OperationsMonth {
  const months = base.year * 12 + base.month + delta;
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

/** Текущий ли это месяц клиентского «сегодня»: правая стрелка навигации
 * гасится на нём — на экранах только paid-операции (резолюция #474),
 * в будущем их не бывает. */
export function isCurrentOperationsMonth(month: OperationsMonth, today: IsoDate): boolean {
  return month.year === Number(today.slice(0, 4))
    && month.month === Number(today.slice(5, 7)) - 1;
}
