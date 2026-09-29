import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  CANON_TIMINGS,
  REDUCED_TIMINGS,
  STALE_MAX_MS,
  initialLiveValueState,
  reduceLiveValue,
  type LiveValueMode,
} from './live-value-machine';

/** Машина последовательности канона C (решение владельца 25.09 по #879,
 * тикет #880): dim (350мс) → hold на фактическое перечитывание (≥300мс) →
 * blur-кроссфейд (220/380) + вспышка (1200). Времена в тестах — от
 * канонных таймингов, чтобы смысл пауз не потерялся. */
const { staleTotalMs, settleMs } = CANON_TIMINGS;

function reduce(state: Parameters<typeof reduceLiveValue>[0], event: Parameters<typeof reduceLiveValue>[1], mode: LiveValueMode = 'crossfade') {
  return reduceLiveValue(state, event, mode, CANON_TIMINGS);
}

const dueAt = (step: ReturnType<typeof reduceLiveValue>): number | null => step.state.dueAt;

describe('live-value machine — канон C: dim → hold → кроссфейд+вспышка (#880)', () => {
  it('idle: старт перечитывания приглушает — фаза dim', () => {
    const next = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    expect(next.state.phase).toBe('stale');
    expect(next.state).toMatchObject({ startedAt: 1000, sawChange: false });
    // Дедлайн dim — страховочный STALE_MAX (смена ещё не приехала).
    expect(dueAt(next)).toBe(1000 + STALE_MAX_MS);
  });

  it('режим B (flash): перечитывание не приглушает — вспышка без dim', () => {
    const next = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 }, 'flash');
    expect(next.state.phase).toBe('idle');
  });

  it('dim без смены значения: конец перечитывания возвращает idle без анимации', () => {
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'refreshEnd', at: 1400 });
    expect(step.state.phase).toBe('idle');
    expect(dueAt(step)).toBeNull();
  });

  it('hold на фактическое перечитывание: быстрая доставка держит dim до 650мс, свап откладывается', () => {
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'change', at: 1200, own: false });
    expect(step.state.phase).toBe('stale');
    expect(step.state).toMatchObject({ sawChange: true });
    step = reduce(step.state, { type: 'refreshEnd', at: 1250 });
    // 1250 < 1000 + 650 — свап ещё рано, hold-дедлайн стоит в состоянии.
    expect(step.state.phase).toBe('stale');
    expect(dueAt(step)).toBe(1000 + staleTotalMs);
    // Таймер сгорел — свап.
    step = reduce(step.state, { type: 'due', at: 1000 + staleTotalMs });
    expect(step.state.phase).toBe('swap');
    expect(step.state.generation).toBe(1);
    expect(dueAt(step)).toBe(1000 + staleTotalMs + settleMs);
  });

  it('медленная доставка: свап сразу по завершении перечитывания — hold уже вычтен', () => {
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'change', at: 1400, own: false });
    step = reduce(step.state, { type: 'refreshEnd', at: 1000 + staleTotalMs + 50 });
    expect(step.state.phase).toBe('swap');
    expect(step.state.generation).toBe(1);
    expect(dueAt(step)).toBe(1000 + staleTotalMs + 50 + settleMs);
  });

  it('прямая смена без перечитывания (влитие в кэш): сразу кроссфейд+вспышка', () => {
    const next = reduce(initialLiveValueState(), { type: 'change', at: 500, own: false });
    expect(next.state.phase).toBe('swap');
    expect(next.state.generation).toBe(1);
    expect(dueAt(next)).toBe(500 + settleMs);
  });

  it('своё изменение — без анимаций в любой фазе', () => {
    expect(reduce(initialLiveValueState(), { type: 'change', at: 500, own: true }).state.phase).toBe('idle');
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'change', at: 1100, own: true });
    expect(step.state.phase).toBe('idle');
    expect(dueAt(step)).toBeNull();
  });

  it('смена значения во время кроссфейда становится в очередь и переигрывает свап', () => {
    let step = reduce(initialLiveValueState(), { type: 'change', at: 500, own: false });
    const firstGeneration = step.state.generation;
    step = reduce(step.state, { type: 'change', at: 700, own: false });
    expect(step.state.phase).toBe('swap');
    expect(step.state).toMatchObject({ pendingChange: true, generation: firstGeneration });
    // Сеттл первой анимации — очередь раскручивает повторный свап.
    step = reduce(step.state, { type: 'due', at: 500 + settleMs });
    expect(step.state.phase).toBe('swap');
    expect(step.state).toMatchObject({ pendingChange: false, generation: firstGeneration + 1 });
    expect(dueAt(step)).toBe(500 + settleMs + settleMs);
    // Второй сеттл без очереди — покой.
    const done = reduce(step.state, { type: 'due', at: 500 + settleMs + settleMs });
    expect(done.state.phase).toBe('idle');
    expect(dueAt(done)).toBeNull();
  });

  it('сеттл без очереди возвращает idle', () => {
    let step = reduce(initialLiveValueState(), { type: 'change', at: 500, own: false });
    step = reduce(step.state, { type: 'due', at: 500 + settleMs });
    expect(step.state.phase).toBe('idle');
    expect(dueAt(step)).toBeNull();
  });

  it('blur-кроссфейд живёт в таймингах машины синхронно токену --live-blur-in', () => {
    // Дрейф-страж: CSS blur-кроссфейд играет var(--live-blur-in), WAAPI-
    // движения размера и резкости в LiveValue.tsx — blurInMs; §8 требует,
    // чтобы размер и резкость доезжали вместе. tokens.css в node-тест не
    // импортируется — читаем файл от этого теста: --live-blur-in живёт
    // в канон-блоке (:root) и reduced-блоке (@media) — других вхождений нет.
    const tokensCss = readFileSync(
      fileURLToPath(new URL('../../styles/tokens.css', import.meta.url)),
      'utf8',
    );
    const blurInMs = [...tokensCss.matchAll(/--live-blur-in:\s*(\d+)ms/g)].map((m) => Number(m[1]));
    expect(blurInMs, 'канон-блок и reduced-блок tokens.css').toEqual([
      CANON_TIMINGS.blurInMs,
      REDUCED_TIMINGS.blurInMs,
    ]);
  });

  it('кривая WAAPI-движений размера — токен --dl-ease посимвольно', () => {
    // Дрейф-страж: WAAPI не принимает var() — кривая --dl-ease продублирована
    // в LiveValue.tsx литералом для движений размера; расхождение токена и
    // литерала разъехало бы размер и резкость, которые §8 требует доезжать
    // вместе. Сравниваем значение токена и литерал посимвольно.
    const tokensCss = readFileSync(
      fileURLToPath(new URL('../../styles/tokens.css', import.meta.url)),
      'utf8',
    );
    const easeValues = [...tokensCss.matchAll(/--dl-ease:\s*(cubic-bezier\([^)]*\))/g)].map(
      (m) => m[1],
    );
    expect(easeValues, 'значение --dl-ease в tokens.css').toHaveLength(1);
    const source = readFileSync(
      fileURLToPath(new URL('./LiveValue.tsx', import.meta.url)),
      'utf8',
    );
    const easingValues = [...source.matchAll(/easing:\s*'([^']+)'/g)].map((m) => m[1]);
    expect(easingValues, 'литерал easing движений размера в LiveValue.tsx').toHaveLength(1);
    expect(easingValues[0]).toBe(easeValues[0]);
  });

  it('reduced-motion укорачивает hold и чистку до ~150мс-порядка', () => {
    expect(REDUCED_TIMINGS.staleTotalMs).toBeLessThanOrEqual(250);
    expect(REDUCED_TIMINGS.settleMs).toBeLessThanOrEqual(300);
    let step = reduceLiveValue(
      initialLiveValueState(),
      { type: 'refreshStart', at: 1000 },
      'crossfade',
      REDUCED_TIMINGS,
    );
    step = reduceLiveValue(step.state, { type: 'change', at: 1100, own: false }, 'crossfade', REDUCED_TIMINGS);
    step = reduceLiveValue(step.state, { type: 'refreshEnd', at: 1150 }, 'crossfade', REDUCED_TIMINGS);
    expect(dueAt(step)).toBe(1000 + REDUCED_TIMINGS.staleTotalMs);
  });
});

describe('live-value machine — самозалечивание stale (#880, грабля живой приёмки)', () => {
  const staleWithChange = () => {
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'change', at: 1100, own: false });
    return step.state;
  };

  it('due в stale со сменой играет свап по hold даже если refreshEnd потерян', () => {
    // Живая приёмка: событие конца перечитывания потеряно — hold-таймер
    // выводит из dim без него.
    const step = reduce(staleWithChange(), { type: 'due', at: 1000 + staleTotalMs });
    expect(step.state.phase).toBe('swap');
    expect(step.state.generation).toBe(1);
  });

  it('due в stale без смены до дедлайна держит dim (перечитывание ещё летит)', () => {
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'due', at: 1000 + staleTotalMs });
    expect(step.state.phase).toBe('stale');
    expect(dueAt(step)).toBe(1000 + STALE_MAX_MS);
  });

  it('due в stale без смены после STALE_MAX поднимает dim — застрявшая фаза невозможна', () => {
    let step = reduce(initialLiveValueState(), { type: 'refreshStart', at: 1000 });
    step = reduce(step.state, { type: 'due', at: 1000 + STALE_MAX_MS });
    expect(step.state.phase).toBe('idle');
    expect(dueAt(step)).toBeNull();
  });

  it('STALE_MAX — страховочный дедлайн dim, канонные hold его не трогают', () => {
    expect(STALE_MAX_MS).toBeGreaterThanOrEqual(4000);
    expect(STALE_MAX_MS).toBeGreaterThan(staleTotalMs * 4);
  });
});
