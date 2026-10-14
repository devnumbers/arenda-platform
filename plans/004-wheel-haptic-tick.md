# 004 — Тик вибрации на каждый ряд колеса (Android)

- **Status**: TODO
- **Commit**: 997e03e97
- **Severity**: LOW (упущенная возможность аудита; решение владельца 2026-10-14 — брать)
- **Category**: Multimodal feedback (Apple §13 — каузальность/гармония/utility)
- **Estimated scope**: 3 файла (новые wheel-haptics.ts + wheel-haptics.test.ts, правки wheel-picker.tsx), ~60 строк

## Problem

Нативный UIPickerView тикает (UISelectionFeedbackGenerator) на каждый
детент-ряд — половина «ощущения колеса». В вебе вибрация доступна только на
Android Chrome (`navigator.vibrate`); iOS Safari её не реализует — там тик
честно превращается в no-op. Сейчас пикер не тикает вовсе.

Решение владельца (2026-10-14): тик на каждый ряд добавить.

## Target

1. Новый чистый модуль `apps/frontend/shared/ui/design/wheel-haptics.ts`
   (паттерн — `wheel-edge.ts`):

```ts
/** Тактильный тик колеса на смену ряда под полосой (решение владельца
 * 2026-10-14; канон Apple §13 — каузальный отклик детента, как
 * UISelectionFeedbackGenerator у UIPickerView). Веб-вибрация есть только
 * на Android Chrome: iOS Safari не реализует navigator.vibrate — тик
 * там честный no-op, desktop без вибромотора молчит. Форма тика:
 * 8мс — короткий «щелчок» детента; пауза между тиками ≥40мс — на
 * быстром флике ряды пролетают чаще, чем вибромотор успевает отыграть
 * щелчок, без паузы они сливаются в сплошной гул. */

/** Минимальная пауза между тиками (мс). */
export const WHEEL_TICK_GAP_MS = 40;

/** Длительность одного тика (мс). */
export const WHEEL_TICK_MS = 8;

/** Пора ли тикать: первый тик всегда, дальше — не чаще GAP. */
export function shouldTickRow(now: number, lastTickAt: number | null): boolean {
  return lastTickAt === null || now - lastTickAt >= WHEEL_TICK_GAP_MS;
}

/** Тик по вибромотору; отсутствие API — тишина (no-op). */
export function tickRow(): void {
  if (typeof navigator !== 'undefined' && typeof navigator.vibrate === 'function') {
    navigator.vibrate(WHEEL_TICK_MS);
  }
}
```

2. Тесты `apps/frontend/shared/ui/design/wheel-haptics.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest';
import { WHEEL_TICK_MS, shouldTickRow, tickRow } from './wheel-haptics';

describe('shouldTickRow', () => {
  it('первый тик всегда', () => {
    expect(shouldTickRow(1000, null)).toBe(true);
  });

  it('чаще паузы — глушится', () => {
    expect(shouldTickRow(1020, 1000)).toBe(false);
  });

  it('по паузе — тикает', () => {
    expect(shouldTickRow(1040, 1000)).toBe(true);
  });
});

describe('tickRow', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    Reflect.deleteProperty(navigator, 'vibrate');
  });

  it('зовёт navigator.vibrate длительностью тика', () => {
    const vibrate = vi.fn();
    Reflect.set(navigator, 'vibrate', vibrate);
    tickRow();
    expect(vibrate).toHaveBeenCalledWith(WHEEL_TICK_MS);
  });

  it('без API — no-op без исключений', () => {
    Reflect.deleteProperty(navigator, 'vibrate');
    expect(() => tickRow()).not.toThrow();
  });
});
```

3. Интеграция в `apps/frontend/shared/ui/design/wheel-picker.tsx`:

   а) Импорт: `import { shouldTickRow, tickRow } from './wheel-haptics';`

   б) Рядом с ref-ами (после `pendingStartRef`):

```tsx
  // Тактильный тик: последний ряд, прошедший полосу (физический), и
  // время прошлого тика — тики не чаще паузы из wheel-haptics.ts.
  const lastTickedRowRef = useRef<number>(
    loop ? WHEEL_LOOP_ANCHOR * items.length + valueIndex : valueIndex,
  );
  const lastTickAtRef = useRef<number | null>(null);
```

   в) В `handleScroll`, в ветке loop-речентра (после
   `setScrollRow(row + jumpRows);`) добавить синхронизацию, чтобы прыжок
   не тикал спуриозно:

```tsx
        lastTickedRowRef.current = row + jumpRows;
```

   г) В конце `handleScroll`, сразу перед `if (settleTimerRef.current !==
   null) …`:

```tsx
    if (row !== lastTickedRowRef.current) {
      lastTickedRowRef.current = row;
      const now = performance.now();
      if (shouldTickRow(now, lastTickAtRef.current)) {
        lastTickAtRef.current = now;
        tickRow();
      }
    }
```

   д) К док-комментарию компонента добавить: «Смена ряда под полосой
   тикает вибромотором (wheel-haptics.ts): Android Chrome — щелчок 8мс
   с паузой 40мс, iOS Safari / desktop — честный no-op.»

## Repo conventions to follow

- Чистые модули колеса с тестами: `wheel-edge.ts` + `wheel-edge.test.ts`.
- Комментарии по-русски, с «почему» и ссылками на решения владельца
  (пример: док-комментарий wheel-edge.ts:1-18).
- Мультимодальный канон (apple-design §13): каузальность — тик на
  фактическую смену ряда под полосой, не на коммит; utility — без
  вибраций на открытие/кнопки.

## Steps

1. **TDD**: `wheel-haptics.test.ts` → красный (`npx vitest run
   wheel-haptics` в apps/frontend).
2. `wheel-haptics.ts` → зелёный.
3. Интеграция в `wheel-picker.tsx` по шагам а)–д).

## Boundaries

- Do NOT тикать на открытие шита, коммит по кнопкам, клик по ряду
  (клик меняет ряд — тик придёт сам через scroll-события ролла).
- Do NOT менять частоту/длину тиков без владельца (8мс/40мс — стартовые,
  правки по итогам feel-check на телефоне).
- Do NOT трогать механику скролла/коммита и другие потребители WheelPicker.
- Если код не совпадает с цитатами (дрейф от коммита 997e03e97) — СТОП
  и отчёт.

## Verification

- **Mechanical**: `npx vitest run wheel-haptics` в apps/frontend — зелёный;
  `make frontend-test` — зелёный.
- **Feel check — только реальный Android-телефон** (эмуляция не валидна,
  канон mobile-native): dev-сервер по LAN-IP, Chrome на телефоне:
  - медленная прокрутка: короткий щелчок на каждый ряд;
  - жёсткий флик: тики не сливаются в гул (пауза 40мс работает); если
    гул — увеличить WHEEL_TICK_GAP_MS до 60 и перепроверить;
  - loop-речентр (часы/минуты, крутить долго в одну сторону) — на
    невидимом прыжке спуриозного тика нет;
  - iPhone (Safari): никакой вибрации, скролл не сломался.
- **Done when**: на Android щелчки по рядам без гула, на iOS no-op,
  `make frontend-test` зелёный.
