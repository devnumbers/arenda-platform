# 007 — use-swap-phase: мгновенная подмена из фазы ухода

- **Status**: DONE
- **Commit**: 613729c87
- **Severity**: LOW
- **Category**: Interruptibility (клик в фазу ухода перезапускает таймер — до 180мс пустого текста при спаме)
- **Estimated scope**: 1 файл (components/use-swap-phase.ts)

## Problem

```ts
// components/use-swap-phase.ts:33-49 — current
const swap = (next: Value) => {
  if (swapTimer.current !== null) {
    window.clearTimeout(swapTimer.current);
  }
  if (phase === "entering") {
    // Клик посреди въезда: старое значение ещё полупрозрачно —
    // подменяем его сразу и переигрываем въезд, не гоня лишнюю
    // фазу ухода.
    setValue(next);
    return;
  }
  setPhase("leaving");
  swapTimer.current = window.setTimeout(() => {
    setValue(next);
    setPhase("entering");
  }, outMs);
};
```

Клик во время `leaving` (текст уже прозрачен) перезапускает полный таймер outMs — текст остаётся пустым до 180мс на каждый спам-клик.

## Target

Из фазы ухода — подмена сразу, въезд переигрывается (симметрично ветке `entering`):

```ts
const swap = (next: Value) => {
  if (swapTimer.current !== null) {
    window.clearTimeout(swapTimer.current);
    swapTimer.current = null;
  }
  if (phase === "entering" || phase === "leaving") {
    // Клик посреди въезда или ухода: значение подменяем сразу и
    // переигрываем въезд — фазу ухода второй раз не гоняем.
    setValue(next);
    setPhase("entering");
    return;
  }
  setPhase("leaving");
  swapTimer.current = window.setTimeout(() => {
    setValue(next);
    setPhase("entering");
  }, outMs);
};
```

Нюанс: в ветке entering раньше был только `setValue(next)` — добавка `setPhase("entering")` идемпотентна (фаза уже entering; перезапись тем же значением не даёт ре-рендера) и делает обе ветки одинаковыми. `key={textStep}` в потребителях переигрывает въезд-анимацию remount'ом; `onAnimationEnd` вернёт idle.

## Repo conventions to follow

- Комментарии-обоснования в файле — стиль сохранён (русский, объясняет «почему»).

## Steps

1. use-swap-phase.ts — заменить тело `swap` на Target (объединить ветки entering/leaving).

## Boundaries

- Do NOT менять outMs/длительности/классы фаз у потребителей (steps.tsx, tariff-cards.tsx).
- Do NOT трогать settle/onAnimationEnd.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Feel check**: спам-клики по шагам 1-4 и по Год/Месяц — текст не пустеет дольше одного короткого ухода; финальное значение совпадает с последней нажатой кнопкой; въезд-анимация всегда доигрывается до конца (idle).
- **Done when**: при спаме текст виден в любой момент, кроме первого ухода 180мс.
