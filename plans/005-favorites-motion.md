# 005 — Favorites: спринг layout-анимаций соседей + домовый useReducedMotion

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: MEDIUM (#5) / LOW (#11)
- **Category**: Interruptibility (жест → спринг), когезия (один источник истины)
- **Estimated scope**: 1 файл

## Problem

`widgets/payments/ui/payments-favorites-screen.tsx`:

- `:655-662` — `Reorder.Item` с `transition={{ duration, ease: EDIT_ROW_EASE }}`:
  ВСЕ анимации элемента, включая layout-анимации соседей при drag-сортировке и
  клавиатурных стрелках, едут фиксированным твином 350мс. Твин не несёт
  скорости: при быстром drag/сериях стрелок соседи отстают от пальца.
  Докстринг файла (`:434`) обещает «соседи пружинят layout-анимацией» — код
  противоречит собственному канону. AUDIT §4: gesture-driven motion — спринг.
- `:6,475` — framer-овский `useReducedMotion` (первый рендер `null`,
  приглушается `?? false`) рядом с домовым
  `shared/lib/hooks/useReducedMotion.ts` (boolean) — два источника истины
  в одном приложении (LiveValue/SnapRail используют домовый).

## Target

```tsx
import { useReducedMotion } from "@/shared/lib/hooks/useReducedMotion";
// …
const reducedMotion = useReducedMotion();
const duration = reducedMotion ? EDIT_ROW_REDUCED_S : EDIT_ROW_LIFT_S;
// …
transition={{
  duration,
  ease: EDIT_ROW_EASE,
  layout: { type: "spring", duration, bounce: 0 },
}}
```

`bounce: 0` = критическое демпфирование (damping 1.0) — спринг без отскока,
несёт скорость через ретаргет; лифт-анимация (scale/тень) остаётся твином
той же длительности. Длительности/кривая не меняются.

## Steps

1. Импорт домового хука вместо framer-овского (оставив `Reorder`,
   `useDragControls` из framer).
2. Добавить `layout: { type: "spring", duration, bounce: 0 }` в transition.
3. Комментарий у EDIT_ROW_* констант дополнить: layout — спринг (velocity
   при ретаргете), лифт — твин.

## Boundaries

- Не трогать механику DnD, клавиатурный порядок, анонсы.
- `whileDrag={lift}` и `animate={grabbed ? lift : rest}` не менять: scale-шортхат
  в Reorder.Item обязателен (полная строка transform затёрла бы трансформ драга).

## Verification

- Механика: e2e `favorites-edit.spec.ts` зелёный.
- Feel-check: быстрый drag вверх-вниз — соседи следуют с ускорением пальца,
  без «отставания» твина; серия ArrowUp/Down — строки догоняют без лага.
