import { describe, expect, it } from 'vitest';
import { snapRailOptions } from './snap-rail-options';

// Движок горизонтальных snap-лент — Embla v9 (тикет #1349, решение
// владельца 13.10 «переходим на Embla dragFree: 'snap'»): кастомная
// pointer-физика удалена, физика Embla принята как база. Чистый шов —
// конфигурация движка (окружение тестов — node без DOM).

describe('snapRailOptions', () => {
  it('полное движение — dragFree "snap" (доезд к ближайшей snap-точке), align по левому краю контейнера, loop выключен', () => {
    expect(snapRailOptions({ reducedMotion: false })).toEqual({
      loop: false,
      align: 'start',
      dragFree: 'snap',
    });
  });

  it('reduced-motion — dragFree false: инерционный доезд гасится, короткий докат к снапу', () => {
    expect(snapRailOptions({ reducedMotion: true })).toEqual({
      loop: false,
      align: 'start',
      dragFree: false,
    });
  });
});
