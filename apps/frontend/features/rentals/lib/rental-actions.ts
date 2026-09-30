import type { Rental } from '@/entities/rental';
import { currentRentalOf } from './rental-view';

/**
 * Состояние-машина «Действий аренды» (карта #984, тикет #986;
 * rentals/CONTEXT.md «Действия аренды»): набор жизненных действий
 * определяет только состояние аренды, и на любой поверхности он один —
 * аренды нет → Создание; «Ожидает начала» → Удаление («передумал
 * до старта»); «идёт» (активная или «Ожидает действия») → Завершение.
 * Машина одна — доменная: страница объекта (#986) и страница аренды
 * (#987) едят один селектор, расхождение поверхностей — баг, а не
 * вариант UX.
 */

export type RentalActionState =
  /** Аренды нет — единственное действие «Начать аренду» (визард). */
  | { readonly kind: 'none' }
  /** «Ожидает начала»: завершения не существует (дата завершения не
   * бывает раньше начала, ADR 0053 §3) — действие «Удалить аренду». */
  | { readonly kind: 'upcoming'; readonly rental: Rental }
  /** «Идёт» — активная или «Ожидает действия»: действие «Завершить
   * аренду» (#627). */
  | { readonly kind: 'active'; readonly rental: Rental };

/** Текущее состояние действий по списку аренд объекта: незавершённую
 * кладёт первой сервер (ADR 0053 §4), завершённые — материал «Прошлых
 * аренд». */
export function rentalActionState(items: ReadonlyArray<Rental>): RentalActionState {
  const rental = currentRentalOf(items);
  if (rental === undefined) {
    return { kind: 'none' };
  }
  return rental.status === 'upcoming'
    ? { kind: 'upcoming', rental }
    : { kind: 'active', rental };
}
