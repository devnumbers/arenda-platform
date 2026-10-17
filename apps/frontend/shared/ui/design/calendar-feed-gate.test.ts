import { describe, expect, it } from 'vitest';
import {
  createPrependGate,
  flingChunks,
  FLING_CHUNKS_CAP,
  shouldDeferPrepend,
} from './calendar-feed-gate';

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

/** flingChunks (план 042, кэп 051): порций на событие по скорости. */
describe('flingChunks — порций на событие по скорости прокрутки', () => {
  const FEED_CHUNK_PX = 2800;

  it('первое событие (нет предыдущего) — одна порция', () => {
    expect(flingChunks(null, 1000, 5000, FEED_CHUNK_PX)).toBe(1);
  });

  it('медленный скролл — одна порция', () => {
    // 40px за 200мс = 0.2px/ms → 0.2×600/2800 < 1
    expect(flingChunks({ st: 5000, t: 1000 }, 1200, 4960, FEED_CHUNK_PX)).toBe(1);
  });

  it('жёсткий флик — кэп 2 (было 8: коммит 48 месяцев душил телефон, план 051)', () => {
    // 3000px за 30мс ≈ 100px/ms — синтетика 042; коммит ограничен 12 месяцами
    expect(flingChunks({ st: 8000, t: 1000 }, 1030, 5000, FEED_CHUNK_PX)).toBe(FLING_CHUNKS_CAP);
    expect(FLING_CHUNKS_CAP).toBe(2);
  });

  it('промежуточная скорость ~6px/ms — 2 порции, кэп не резал бы и 3', () => {
    // 180px за 30мс = 6px/ms → ceil(6×600/2800) = 2
    expect(flingChunks({ st: 8000, t: 1000 }, 1030, 7820, FEED_CHUNK_PX)).toBe(2);
  });

  it('время не поехало (тот же кадр) — одна порция', () => {
    expect(flingChunks({ st: 8000, t: 1000 }, 1000, 5000, FEED_CHUNK_PX)).toBe(1);
  });
});

/** shouldDeferPrepend (план 052): DOM не трогается на высокой скорости. */
describe('shouldDeferPrepend — моментум-окно приписки', () => {
  it('первое событие — не откладываем', () => {
    expect(shouldDeferPrepend(null, 1000, 5000)).toBe(false);
  });

  it('медленный драг — пускаем', () => {
    // 30px за 200мс = 0.15px/ms
    expect(shouldDeferPrepend({ st: 5000, t: 1000 }, 1200, 4970)).toBe(false);
  });

  it('флик — откладываем', () => {
    // 300px за 50мс = 6px/ms — реальный пик флика
    expect(shouldDeferPrepend({ st: 5000, t: 1000 }, 1050, 4700)).toBe(true);
  });

  it('хвост затухания — пускаем', () => {
    // 20px за 30мс ≈ 0.67px/ms
    expect(shouldDeferPrepend({ st: 1000, t: 1000 }, 1030, 980)).toBe(false);
  });
});
