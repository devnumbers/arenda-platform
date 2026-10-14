# 017 — PullToRefresh: спиннер прямыми style-записями, без ре-рендера на каждый touchmove

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: LOW
- **Category**: Performance (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 1 файл (PullToRefresh.tsx), ~25 строк

## Problem

Контент-трансформ PTR пишется мимо React — прямой style-записью на кадр
(внутренний эталон файла), а reveal спиннера (opacity/scale) едет через
React-состояние: `setPullY` + `setPhase` на каждый touchmove жеста —
ре-рендер и reconcile на пути пальца:

```tsx
// apps/frontend/shared/ui/pull-to-refresh/PullToRefresh.tsx:266-269 — current
            pullRef.current = visual;
            setPullY(visual);
            setPhase(visual >= PTR_THRESHOLD_PX ? 'armed' : 'pulling');
            // Content follows the finger frame-by-frame.
            applyContentTransform(visual);
```

```tsx
// PullToRefresh.tsx:340-350 — current (reveal из состояния на рендере)
    const reveal = spinnerReveal(pullY);
    const spinnerStyle: CSSProperties =
        phase === 'refreshing'
            ? {}
            : {
                  opacity: reveal,
                  transform: `translate(-50%, 0) scale(${reveal})`,
              };
```

Жестовое движение — законно JS/спринговое (канон §5), дорога только
обёртка React-рендера. iOS-only поверхность (ADR 0031), поэтому LOW.

## Target

Спиннер — в тот же прямой путь, что контент: ref + запись в существующем
кадровом апдейте; React-состояние остаётся только на фазовых переходах
(редкие события: pull→armed→refreshing→idle), не на каждый move:

```tsx
// PullToRefresh.tsx — target: реф спиннера рядом с contentRef
    const spinnerRef = useRef<HTMLSpanElement>(null);

    // Reveal спиннера — прямыми записями (эталон applyContentTransform):
    // opacity/scale следуют пальцу, React-рендер на move не тратится.
    const applySpinnerReveal = useCallback((reveal: number): void => {
        const el = spinnerRef.current;
        if (!el) return;
        el.style.opacity = String(reveal);
        el.style.transform = `translate(-50%, 0) scale(${reveal})`;
    }, []);
```

```tsx
// в touchmove (рядом с applyContentTransform(visual)) — target
            pullRef.current = visual;
            setPhase(visual >= PTR_THRESHOLD_PX ? 'armed' : 'pulling');
            applyContentTransform(visual);
            applySpinnerReveal(spinnerReveal(visual));
```

(`setPullY` уходит из move-пути; `pullY`-состояние удаляется, если больше
никем не читается — проверить grep по файлу; фазы — через существующий
`phaseRef` + `setPhase`, который в move уже и так дёшево no-op при том же
значении.)

Сброс/финиш — обнулить reveal там же, где контент сбрасывается:

```tsx
// в reset()/финиш-ветке (где applyContentTransform(0)) — target
            applyContentTransform(0);
            applySpinnerReveal(0);
```

Рендер спиннера — без inline style:

```tsx
// PullToRefresh.tsx:349-350 — target
    const spinnerClass = clsx(styles.spinner, phase === 'refreshing' && styles.refreshing);

    return (
        <span ref={spinnerRef} className={spinnerClass} aria-hidden="true">
```

(CSS-базовое состояние `.spinner` — `opacity: 0; transform:
translate(-50%,0) scale(0)` — уже совпадает с reveal 0; при
`phase === 'refreshing'` класс `.refreshing` ведёт fade-in до полной —
существующее поведение модуля не трогается.)

## Repo conventions to follow

- Эталон — сам же файл: `applyContentTransform`
  (`PullToRefresh.tsx:130-140`) — style-записи на кадр, transition не
  участвует, «Springs drive the value per frame».
- React-состояние — только для фаз (как сейчас `phase`); частые значения —
  refs + style.

## Steps

1. Добавить `spinnerRef` + `applySpinnerReveal` (Target).
2. Move-путь: убрать `setPullY`, добавить `applySpinnerReveal` (Target).
3. Сброс: `applySpinnerReveal(0)` рядом со сбросом контента (Target).
4. Рендер: реф на спан, inline-style удалить; удалить `pullY`-состояние,
   если читателей не осталось.

## Boundaries

- Do NOT трогать спринги, `pull-math.ts`, пороги, flick-детект, гард
  overscroll — только путь доставки reveal.
- Do NOT трогать `.spinner`/`.refreshing` CSS.
- `phase` остаётся React-состоянием (нужен для классов/логики).
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные; `make frontend-e2e TESTS='-g pull-to-refresh'` — зелёные.
- **Feel check** (iOS standalone, реальное устройство по протоколу карты):
  - тяга с верха: спиннер проявляется прогрессом тяги как раньше, порог
    «armed» щёлкает вовремя;
  - flick-запуск с малой тяги: fade-in до полной через класс .refreshing —
    без рывка;
  - DevTools (симуляция): тач-драг — ре-рендеров компонента на каждый move
    нет (React DevTools highlight updates), только фазовые;
  - release ниже порога — спиннер гаснет вместе с контентом.
- **Done when**: move-путь не вызывает setState на каждый евент; feel
  идентичен коммиту 2d6925b76.
