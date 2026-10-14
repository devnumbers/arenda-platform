# 003 — Свой ролл программной докрутки (клавиатура/внешнее значение)

- **Status**: TODO
- **Commit**: 997e03e97
- **Severity**: MEDIUM
- **Category**: Interruptibility (Apple §3 — из презентационного значения, прерываемо; + reduced-motion)
- **Estimated scope**: 3 файла (новые wheel-roll.ts + wheel-roll.test.ts, правки wheel-picker.tsx), ~90 строк

## Problem

Программная докрутка колеса — клавиатура и внешняя смена значения — едет
нативным `behavior: 'smooth'`:

```tsx
// apps/frontend/shared/ui/design/wheel-picker.tsx:255-260 — current
    if (Math.round(list.scrollTop / WHEEL_ROW_HEIGHT) !== targetRow) {
      list.scrollTo({
        top: targetRow * WHEEL_ROW_HEIGHT,
        behavior: instantRef.current ? 'auto' : 'smooth',
      });
    }
```

Нативный smooth: длительность и кривая на усмотрение браузера (не токен
`--dl-ease`), ретаргет при удержании стрелки в Safari дёргается, прервать
анимацию жестом нельзя (анимация принадлежит браузеру). Apple §3: анимировать
от презентационного значения и быть прерываемым в любой момент. Плюс ролл не
глушится `prefers-reduced-motion` (находка 6 аудита).

## Target

1. Новый чистый модуль `apps/frontend/shared/ui/design/wheel-roll.ts` —
   длительность и кривая ролла; паттерн — как `wheel-edge.ts`
   (русский док-комментарий с «почему», экспорт констант и чистых функций):

```ts
/** Математика программного ролла колеса (#аудит-анимаций 2026-10-14):
 * клавиатура и внешняя смена значения докручиваются собственной
 * анимацией, а не нативным behavior:'smooth' — длительность и кривая
 * должны быть нашими (--dl-ease), старт — от презентационного значения
 * (канон Apple §3), а сам ролл — прерываемым жестом в любой момент.
 * Длительность растёт с дистанцией, но зажата: один ряд — быстрый
 * щелчок 120мс (бюджет кнопки из AUDIT.md), дальние прыжки (Home/End)
 * не дольше 450мс. Кривая — токен --dl-ease
 * (cubic-bezier(0.32, 0.72, 0, 1)), тот же, что у всех transition слоя. */

/** Нижний порог длительности ролла (мс): короткая докрутка не должна
 * ощущаться задержкой. */
export const WHEEL_ROLL_MIN_MS = 120;

/** Верхний порог длительности ролла (мс): дальний прыжок не катит долго. */
export const WHEEL_ROLL_MAX_MS = 450;

/** Множитель дистанции: 240px (5 рядов) ≈ 144мс. */
const WHEEL_ROLL_MS_PER_PX = 0.6;

/** Длительность ролла для дистанции в пикселях (зажата в [MIN, MAX]). */
export function rollDurationMs(distancePx: number): number {
  const raw = Math.abs(distancePx) * WHEEL_ROLL_MS_PER_PX;
  return Math.max(WHEEL_ROLL_MIN_MS, Math.min(WHEEL_ROLL_MAX_MS, raw));
}

/** Значение кубической кривой Безье (x1,y1,x2,y2) при прогрессе x:
 * решаем параметр t бинарным поиском по X-компоненте, возвращаем
 * Y-компоненту. Достаточно 24 итераций: точность ~1e-7 при диапазоне
 * t ∈ [0,1]; контрольные точки X в [0,1] — поиск сходится всегда. */
export function rollEase(
  x: number,
  x1 = 0.32,
  y1 = 0.72,
  x2 = 0,
  y2 = 1,
): number {
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  const at = (t: number): [number, number] => {
    const omt = 1 - t;
    const xB = 3 * omt * omt * t * x1 + 3 * omt * t * t * x2 + t * t * t;
    const yB = 3 * omt * omt * t * y1 + 3 * omt * t * t * y2 + t * t * t;
    return [xB, yB];
  };
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 24; i += 1) {
    const mid = (lo + hi) / 2;
    const [xB] = at(mid);
    if (xB < x) lo = mid;
    else hi = mid;
  }
  const [, yB] = at((lo + hi) / 2);
  return yB;
}
```

2. Тесты `apps/frontend/shared/ui/design/wheel-roll.test.ts` — по образцу
   `wheel-edge.test.ts` (vitest,describe/it/expect):

```ts
import { describe, expect, it } from 'vitest';
import { WHEEL_ROLL_MAX_MS, WHEEL_ROLL_MIN_MS, rollDurationMs, rollEase } from './wheel-roll';

describe('rollDurationMs', () => {
  it('короткая докрутка — нижний порог', () => {
    expect(rollDurationMs(48)).toBe(WHEEL_ROLL_MIN_MS);
    expect(rollDurationMs(0)).toBe(WHEEL_ROLL_MIN_MS);
  });

  it('длительность растёт с дистанцией', () => {
    expect(rollDurationMs(240)).toBeGreaterThan(rollDurationMs(48));
    expect(rollDurationMs(240)).toBeCloseTo(144, 0);
  });

  it('дальний прыжок зажат верхним порогом, знак не важен', () => {
    expect(rollDurationMs(5000)).toBe(WHEEL_ROLL_MAX_MS);
    expect(rollDurationMs(-5000)).toBe(WHEEL_ROLL_MAX_MS);
  });
});

describe('rollEase (cubic-bezier(0.32, 0.72, 0, 1) = --dl-ease)', () => {
  it('края точны', () => {
    expect(rollEase(0)).toBe(0);
    expect(rollEase(1)).toBe(1);
  });

  it('монотонна и выше диагонали в начале (ease-out характер)', () => {
    let prev = 0;
    for (let i = 1; i <= 20; i += 1) {
      const x = i / 20;
      const y = rollEase(x);
      expect(y).toBeGreaterThanOrEqual(prev);
      prev = y;
    }
    expect(rollEase(0.25)).toBeGreaterThan(0.25);
    expect(rollEase(0.5)).toBeGreaterThan(0.5);
  });
});
```

3. Интеграция в `apps/frontend/shared/ui/design/wheel-picker.tsx`:

   а) Импорты: `import { rollDurationMs, rollEase } from './wheel-roll';`

   б) Рядом с существующими ref-ами добавить контроллер ролла (rAF,
   отмена, reduced-motion):

```tsx
  // Ролл программной докрутки: rAF-анимация от презентационного значения
  // (канон Apple §3), прерываемая жестом человека. scroll-события самого
  // ролла прерыванием не считаются — их пишет анимация.
  const rollFrameRef = useRef<number | null>(null);
  const cancelRoll = (): void => {
    if (rollFrameRef.current !== null) {
      window.cancelAnimationFrame(rollFrameRef.current);
      rollFrameRef.current = null;
    }
  };
  const rollTo = (list: HTMLUListElement, top: number): void => {
    cancelRoll();
    if (
      list.scrollTop === top ||
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    ) {
      list.scrollTop = top;
      return;
    }
    const from = list.scrollTop;
    const duration = rollDurationMs(top - from);
    const startedAt = performance.now();
    const step = (now: number): void => {
      const progress = Math.min((now - startedAt) / duration, 1);
      list.scrollTop = from + (top - from) * rollEase(progress);
      rollFrameRef.current =
        progress < 1 ? window.requestAnimationFrame(step) : null;
    };
    rollFrameRef.current = window.requestAnimationFrame(step);
  };
```

   в) Существующий effect размонтирования (wheel-picker.tsx:181-186)
   дополнить `cancelRoll()`.

   г) В effect синхронизации заменить smooth-скролл (цитата из Problem)
   на:

```tsx
    if (Math.round(list.scrollTop / WHEEL_ROW_HEIGHT) !== targetRow) {
      if (instantRef.current) {
        list.scrollTop = targetRow * WHEEL_ROW_HEIGHT;
      } else {
        rollTo(list, targetRow * WHEEL_ROW_HEIGHT);
      }
    }
```

   д) Прямая установка при перекройке ленты (wheel-picker.tsx:236,
   `list.scrollTop = valueIndex * WHEEL_ROW_HEIGHT;`) — предварить
   `cancelRoll();` (иначе следующий кадр ролла перезапишет установку).

   е) На `<ul>` добавить обработчики прерывания жестом (к строке
   `onScroll={handleScroll}`):

```tsx
        onPointerDown={cancelRoll}
        onWheel={cancelRoll}
        onTouchStart={cancelRoll}
```

   ж) К блоку док-комментария компонента добавить абзац: «Программная
   докрутка (клавиатура, внешнее значение) — собственный rAF-ролл
   (wheel-roll.ts): старт от текущей позиции, кривая --dl-ease,
   prefers-reduced-motion сажает ряд мгновенно; жест человека
   (pointerdown/wheel/touchstart) ролл обрывает.»

## Repo conventions to follow

- Чистые модули колеса с тестами: `wheel-edge.ts` + `wheel-edge.test.ts`
  (русский док-комментарий с физикой/«почему», константы наружу) —
  wheel-roll.ts повторяет паттерн.
- Кривая движения слоя — токен `--dl-ease` (globals.css:301, tokens.css:101);
  в JS-анимации значения контрольных точек (0.32, 0.72, 0, 1) дублируются
  с комментарием-привязкой к токену (CSS-переменную в canvas-Безье не
  подставить).
- rAF через `window.requestAnimationFrame` / `window.cancelAnimationFrame`
  — как settle-таймеры через `window.setTimeout` (wheel-picker.tsx:183,313).

## Steps

1. **TDD**: написать `wheel-roll.test.ts` (секция Target п.2), убедиться,
   что красный (`npx vitest run wheel-roll` в apps/frontend).
2. Написать `wheel-roll.ts` (секция Target п.1), тесты зелёные.
3. Интегрировать в `wheel-picker.tsx` по шагам а)–ж).

## Boundaries

- Do NOT трогать мгновенные пути: первая установка (`instantRef`),
  ре-анкор loop, установка при перекройке (кроме добавления `cancelRoll`),
  клик по ряду (коммит + ролл через общий effect — уже покрывается).
- Do NOT трогать скролл-механику: snap, инерцию, зону края (wheel-edge.ts),
  loop-речентр (wheel-loop.ts).
- Do NOT менять e2e (`apps/frontend/e2e/wheel-picker.spec.ts`) — они гейт.
- Do NOT добавлять зависимости (никаких spring-библиотек).
- Если правки из п.3 не совпадают с цитатами (дрейф от коммита 997e03e97) —
  СТОП и отчёт.

## Verification

- **Mechanical**:
  - `npx vitest run wheel-roll` в apps/frontend — зелёный;
  - `make frontend-test` — зелёный;
  - `make frontend-e2e TESTS='-g WheelPicker'` (Docker) — все 10 тестов
    зелёные. **Это главный гейт риска**: пер-кадровые записи `scrollTop`
    в контейнер со `scroll-snap-type: y mandatory` браузер может
    реснапить посреди ролла. Если e2e падает (ролл ломается о снап) —
    СТОП: откатить шаг 3г) к `behavior: 'smooth'` и доложить (нативный
    smooth — проверенный фолбэк), не чинить импровизацией.
- **Feel check** (витрина `/ui-kit`, секция колёс, клавиатура):
  - ArrowDown/ArrowUp — ряд докручивается быстро (~120мс) на кривой
    ease-out без «доезда» нативного smooth;
  - удержание стрелки — серия коротких роллов без рывков и пауз
    (ретаргет от текущей позиции);
  - во время ролла кликнуть/потянуть ленту — ролл обрывается мгновенно,
    инерция жеста не конфликтует;
  - Rendering → `prefers-reduced-motion: reduce` → стрелки ставят ряд
    мгновенно, без анимации.
- **Done when**: e2e зелёные, ролл короткий/прерываемый, reduced-motion
  мгновенный.
