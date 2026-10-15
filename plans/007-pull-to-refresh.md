# 007 — PTR: императивный спиннер + зеркальный фейд выхода

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: MEDIUM (#7) / LOW (#8)
- **Category**: Performance (покадровый setState в жесте), Interruptibility
- **Estimated scope**: 2 файла

## Problem

`shared/ui/pull-to-refresh/PullToRefresh.tsx`:

- `:266-268` — `setPullY(visual)` на каждом `touchmove` (до 120/с на
  ProMotion) ради reveal-стиля спиннера, при том что трансформ контента уже
  пишется императивно (`applyContentTransform`). React-ре-рендер — в жестовом
  критическом пути PWA.
- `PullToRefresh.module.css:13-30` + `tsx:199-202` — вход спиннера в
  refreshing: fade 250мс (сознательно, против «попа»), выход — жёсткий кат
  inline `opacity:0/scale(0)`: асимметрия вход/выход, спиннер висит на
  `position: fixed` всю пружину возврата и исчезает мгновенно.

## Target

1. Reveal спиннера — императивно, тем же путём, что контент:

```tsx
/* вместо setPullY: */
const spinner = spinnerRef.current;
if (spinner !== null) {
    const reveal = spinnerReveal(visual);
    spinner.style.opacity = String(reveal);
    spinner.style.transform = `translate(-50%, 0) scale(${reveal})`;
}
```

`setPullY` удалить вместе с потребителем (стиль спиннера больше не из state);
`setPhase` остаётся ТОЛЬКО на смене фазы (armed/pulling/refreshing) — React
сам бейcаутит одинаковые значения. Канон ADR 0031 (жест — императивный путь)
соблюдается обеими линиями.

2. Выход — зеркальный короткий фейд на `reset()`: WAAPI-фейд 150мс
   (fade-семейство, reduced-motion дружелюбен) c `cancel()` предыдущего
   и установкой скрытого состояния по завершении:

```tsx
spinner.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 150, easing: 'ease-out' })
```

## Steps

1. `PullToRefresh.tsx`: `spinnerRef`; reveal-запись рядом с
   `applyContentTransform`; `setPullY` снести (проверить потребителя state).
2. `reset()`: перед скрытием — WAAPI-фейд 150мс; отмена незавершённого фейда
   при новом pull.
3. CSS `.refreshing` transition не трогать (вход остаётся 250мс на токенах).

## Boundaries

- Физику жеста (резинка, спринг, порог 70, flick-решение) не трогать.
- ARIA-анонсы и фазы не менять.

## Verification

- Механика: e2e PTR-спеки зелёные; React DevTools highlight — при жесте
  нет покадровых ре-рендеров компонента.
- Feel-check на телефоне: жест плавный; по отпусканию спиннер коротко
  растворяется синхронно с возвратом контента.
