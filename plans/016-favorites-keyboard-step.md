# 016 — Избранное: клавиатурная перестановка без 350ms-ожидания на шаг

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Purpose & frequency (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 1 файл (payments-favorites-screen.tsx), ~15 строк

## Problem

Режим правки избранного на клавиатуре: каждое нажатие стрелки запускает
перелёт строки, длительность которого наследует лифт-тайминг 0.35s —
серия из 5–10 стрелок ждёт отыгрывания полного tween на каждый шаг. Канон
(AUDIT.md §1): «animations on keyboard-initiated actions» — в списке охоты,
повторяющиеся клавиатурные действия — «drastically reduce».

```tsx
// apps/frontend/widgets/payments/ui/payments-favorites-screen.tsx:605-606 — current
  const duration = reducedMotion ? EDIT_ROW_REDUCED_S : EDIT_ROW_LIFT_S;
```

```tsx
// payments-favorites-screen.tsx:636-643 — current (стрелки двигают строку)
    if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      event.preventDefault();
      const to = event.key === "ArrowUp" ? index - 1 : index + 1;
      if (to >= 0 && to < total) {
        onMoved(to);
      }
      return;
    }
```

```tsx
// payments-favorites-screen.tsx:656-660 — current (единый transition на лифт и перелёт)
      transition={{
        duration,
        ease: EDIT_ROW_EASE,
      }}
```

Лифт при захвате (scale+тень, 0.35s) — честный фидбек состояния, задокументирован
(DESIGN.md §8, константы с комментарием «Движение — по кривой дома:
350ms, reduced-motion — 150ms») и не пересматривается. Сокращается только
шаг перестановки, инициированный с клавиатуры.

## Target

```tsx
// payments-favorites-screen.tsx:449-450 — target (рядом с EDIT_ROW_LIFT_S)
/** Шаг перестановки стрелками: клавиатурное повторяющееся действие —
 * канон §1 «drastically reduce»; лифт и drag не трогает. */
const EDIT_ROW_KEYBOARD_S = 0.15;
```

Локальный признак «захват с клавиатуры» + ветвление тайминга:

```tsx
// внутри компонента FavoritesEditRow — target
  const [keyboardGrab, setKeyboardGrab] = useState(false);

  const duration = reducedMotion
    ? EDIT_ROW_REDUCED_S
    : keyboardGrab
      ? EDIT_ROW_KEYBOARD_S
      : EDIT_ROW_LIFT_S;
```

Захват/снятие захвата с клавиатуры (существующий keydown-обработчик):

```tsx
// ветка Space/Enter — target
      if (!event.repeat) {
        setKeyboardGrab(!grabbed);
        onGrabChange(!grabbed);
      }
      return;
```

```tsx
// ветка Escape — target
    if (event.key === "Escape") {
      event.preventDefault();
      setKeyboardGrab(false);
      onGrabChange(false);
    }
```

Синхронизация при захвате мышью/тачем (grab-состояние приходит из пропов):

```tsx
// рядом с duration — target
  useEffect(() => {
    if (!grabbed) setKeyboardGrab(false);
  }, [grabbed]);
```

Тач-драг и лифт не меняются: `whileDrag={lift}` и захват указателем идут с
`keyboardGrab === false` → 0.35s как задокументировано.

## Repo conventions to follow

- Зеркала токенов движения — задокументированные константы файла
  (`EDIT_ROW_LIFT_S/REDUCED_S/EASE` с комментариями, 0.35/0.15) — новая
  константа в тот же блок, с тем же стилем комментария.
- Канон §1: клавиатурные повторяющиеся действия — минимальная анимация;
  0.15s = `EDIT_ROW_REDUCED_S` (уже используемое «короткое» значение файла).

## Steps

1. Константа `EDIT_ROW_KEYBOARD_S` (Target, блок констант ~449).
2. `keyboardGrab`-состояние, ветвление `duration`, две ветки keydown,
   sync-эффект (Target).

## Boundaries

- Do NOT трогать лифт (0.35s), тень, пружины соседей, drag — только тайминг
  шага при клавиатурном захвате.
- Do NOT менять e2e `favorites-edit.spec.ts` — механика перестановки та же.
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные; `make frontend-e2e TESTS='-g избранн'` — зелёные.
- **Feel check** (десктоп, клавиатура): захват пробелом → 5 стрелок подряд
  (в том числе зажатой) — шаги идут быстро, без ожидания tween; тень/лифт
  при захвате — как раньше; захват мышью за ручку — 0.35s лифт не изменился;
  Escape сбрасывает; повторный захват стрелками после тач-захвата — снова
  быстрый.
- **Done when**: шаг стрелки ≈150ms при клавиатурном захвате; drag/лифт
  неизменны.
