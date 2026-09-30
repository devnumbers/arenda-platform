import { describe, expect, it } from 'vitest';
import { makeRental } from '@/entities/rental';
import { rentalActionState } from './rental-actions';

/**
 * Селектор «Действий аренды» (карта #984, тикет #986; rentals/CONTEXT.md
 * «Действия аренды»): набор действий определяет состояние, машина одна —
 * доменная, поверхности синхронны. Страница объекта ест его с #986,
 * страница аренды — наследник (#987).
 */
describe('rentalActionState (состояния «Действий аренды»)', () => {
  it('пустой список — аренды нет (действие — Создание)', () => {
    expect(rentalActionState([])).toEqual({ kind: 'none' });
  });

  it('только завершённые — аренды нет: они материал «Прошлых аренд»', () => {
    const completed = makeRental({ id: 'r1', status: 'completed', completedDate: '2026-09-01' });
    expect(rentalActionState([completed])).toEqual({ kind: 'none' });
  });

  it('«Ожидает начала» (upcoming) — действие Удаление («передумал до старта»)', () => {
    const upcoming = makeRental({ id: 'r2', status: 'upcoming', startDate: '2026-10-10' });
    expect(rentalActionState([upcoming])).toEqual({ kind: 'upcoming', rental: upcoming });
  });

  it('активная — «идёт»: действие Завершение', () => {
    const active = makeRental({ id: 'r3', status: 'active' });
    expect(rentalActionState([active])).toEqual({ kind: 'active', rental: active });
  });

  it('«Ожидает действия» (needs_attention) — тоже «идёт»: Завершение (#627)', () => {
    const attention = makeRental({ id: 'r4', status: 'needs_attention' });
    expect(rentalActionState([attention])).toEqual({ kind: 'active', rental: attention });
  });

  it('сервер кладёт незавершённую первой (ADR 0053 §4) — завершённые в хвосте не мешают', () => {
    const completed = makeRental({ id: 'r5', status: 'completed', completedDate: '2026-09-01' });
    const upcoming = makeRental({ id: 'r6', status: 'upcoming' });
    expect(rentalActionState([completed, upcoming])).toEqual({
      kind: 'upcoming',
      rental: upcoming,
    });
  });
});
