# 002 — Непрерывное потемнение ряда в SDA-каскаде

- **Status**: TODO
- **Commit**: 997e03e97
- **Severity**: MEDIUM
- **Category**: Physicality (непрерывный отклик, Apple §1/§2)
- **Estimated scope**: 1 файл (globals.css), 6 строк

## Problem

Барабан-каскад WheelPicker (`wheel-row-cascade`, scroll-driven animation)
непрерывно по позиции скролла меняет `scale`/`opacity` ряда, но цвет текста
переключается бинарно — React-состоянием `dark` на границе полосы выбора
(`apps/frontend/shared/ui/design/wheel-picker.tsx:128-129`), сглаженным лишь
`transition-colors` (150мс). На SDA-браузерах ряд у края полосы (opacity
≈0.57) «щёлкает» цветом — ступенька на полупрозрачном ряду видна. iOS темнит
ряд непрерывно к центру.

Текущие кейфреймы:

```css
/* apps/frontend/app/globals.css:1051-1063 — current */
@keyframes wheel-row-cascade {
    from {
        scale: 0.72;
        opacity: 0.35;
    }

    50% {
        scale: 1;
        opacity: 1;
    }

    to {
        scale: 0.72;
        opacity: 0.35;
    }
}
```

## Target

```css
/* apps/frontend/app/globals.css:1051 — target */
@keyframes wheel-row-cascade {
    from {
        scale: 0.72;
        opacity: 0.35;
        color: var(--color-content-secondary);
    }

    50% {
        scale: 1;
        opacity: 1;
        color: var(--color-content);
    }

    to {
        scale: 0.72;
        opacity: 0.35;
        color: var(--color-content-secondary);
    }
}
```

Плюс дополнить общий комментарий блока (globals.css:1044-1050) одним
предложением: «Цвет ряда в тех же кейфреймах — потемнение к центру
непрерывное, без ступеньки на границе полосы; React-переключение `dark`
(wheel-picker.tsx) остаётся фолбэком для браузеров без SDA.»

Механика: CSS-анимация (даже scroll-driven) стоит в каскаде выше обычных
авторских правил, поэтому на SDA-браузерах анимированный `color` побеждает
классы `text-content`/`text-content-secondary`; вне `@supports
(animation-timeline: view())` кейфреймы не исполняются и классовый фолбэк
работает как прежде. Селекция `aria` не зависит от цвета — её не трогаем.

## Repo conventions to follow

- Цветовые переменные слоя: `--color-content: var(--dl-text-primary)`,
  `--color-content-secondary: var(--dl-text-secondary)`
  (apps/frontend/app/globals.css:238-239) — в кейфреймах используются эти
  `--color-*` имена, как их используют утилиты `text-content` /
  `text-content-secondary`.
- Значения scale/opacity (0.72 / 0.35 / 1) не менять — это решение владельца
  2026-10-13 («барабан-каскад»).

## Steps

1. `apps/frontend/app/globals.css`: добавить `color` в три ключевых кадра
   `wheel-row-cascade` (см. Target).
2. Там же: дополнить комментарий блока предложением из Target.

## Boundaries

- Do NOT трогать `wheel-picker.tsx` (классы `dark`/`transition-colors` —
  фолбэк, остаются).
- Do NOT менять scale/opacity значения, таймлайн и `@supports`-обвязку.
- Do NOT трогать прочие кейфреймы файла (modal-*, hub-*).
- Если кейфреймы не совпадают с цитатой (дрейф от коммита 997e03e97) — СТОП
  и отчёт.

## Verification

- **Mechanical**: `make frontend-test` — зелёный; визуальная регрессия на
  браузере без SDA (Firefox ≤143 или DevTools-эмуляция без `view()`) —
  ряды плоские, фолбэк-цвет по-прежнему переключается на границе полосы.
- **Feel check** (Chrome 115+ / Safari 26, витрина `/ui-kit`, секция колёс):
  - медленная прокрутка колеса: ряд темнеет постепенно по мере подхода к
    центру, ступеньки цвета на границе полосы нет;
  - быстрый флик: каскад scale/opacity не изменился, рывков цвета нет;
  - во вкладке Rendering включить `prefers-reduced-motion: reduce` —
    каскад живёт (он позиционный — управление, не анимация; канон §8).
- **Done when**: потемнение непрерывно на SDA-браузерах, фолбэк без
  изменений на остальных, `make frontend-test` зелёный.
