import { describe, expect, it } from 'vitest';
import { createPrependGate } from './calendar-feed-gate';

/** Гейт приписки (план 050): одна порция «в полёте» — скролл-события
 * одного кадра не суммируются в один DOM-коммит. */
describe('createPrependGate — одна приписка в полёте', () => {
  it('первый запрос проходит: порция применяется', () => {
    const gate = createPrependGate();
    expect(gate.request(6)).toBe(6);
  });

  it('запросы до settled склеиваются: вместо трёх порций — одна', () => {
    // Три скролл-события одного кадра: первое запрашивает приписку,
    // остальные два получают null — React применит только первую.
    const gate = createPrependGate();
    expect(gate.request(1)).toBe(1);
    expect(gate.request(1)).toBeNull();
    expect(gate.request(1)).toBeNull();
  });

  it('settled открывает следующий запрос: редкие события (душащий CPU) не склеиваются', () => {
    // Сценарий 042: события доезжают с лагом в сотни мс — к каждому
    // предыдущая порция уже уехала, гейт пропускает каждую.
    const gate = createPrependGate();
    expect(gate.request(8)).toBe(8);
    gate.settled();
    expect(gate.request(1)).toBe(1);
    gate.settled();
    expect(gate.request(2)).toBe(2);
  });

  it('пустой запрос порции не открывает полёт', () => {
    const gate = createPrependGate();
    expect(gate.request(0)).toBeNull();
    expect(gate.pending).toBe(false);
    expect(gate.request(6)).toBe(6);
  });

  it('pending отражает полёт', () => {
    const gate = createPrependGate();
    expect(gate.pending).toBe(false);
    gate.request(6);
    expect(gate.pending).toBe(true);
    gate.settled();
    expect(gate.pending).toBe(false);
  });
});
