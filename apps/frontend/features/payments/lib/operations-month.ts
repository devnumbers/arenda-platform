import type { IsoDate } from '@/entities/payment';

/**
 * Календарный месяц в модели операций: month — 0..11, как у Date.
 * Потребители — месячная сводка секции «Операции в <месяц>» на странице
 * объекта (#473 — сознательно осталась месячной) и её месячный диапазон.
 * «Сегодня» — клиентское (та же оговорка про TZ: зоны смотрящего сервер
 * не сообщает, расхождение с TZ собственника ограничено краевыми часами).
 */
export type OperationsMonth = {
  readonly year: number;
  readonly month: number;
};

/** Месяц, в который попадает ISO-дата «сегодня». */
export function operationsMonthOf(today: IsoDate): OperationsMonth {
  return { year: Number(today.slice(0, 4)), month: Number(today.slice(5, 7)) - 1 };
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
