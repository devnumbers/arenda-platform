# 002 — Ленты: will-change на время движения, не навсегда

- **Status**: DONE
- **Commit**: 613729c87
- **Severity**: MEDIUM
- **Category**: Performance/GPU (6 перманентных композитных слоёв на планшете; постоянно удержанные тайлы ~5-15МБ)
- **Estimated scope**: 3 файла (components/strip-engine.ts, components/sections/rentals-carousel.module.css, components/tablet-strip.module.css)

## Problem

Дорожки лент держат слой промоушенным навсегда:

```css
/* components/sections/rentals-carousel.module.css:20 — current */
.track {
  display: flex;
  gap: 12px;
  height: 100%;
  width: max-content;
  will-change: transform; /* ← перманентно */
  padding-left: 24px;
}
```

```css
/* components/tablet-strip.module.css:41 — current */
.track {
  display: flex;
  gap: 12px;
  width: max-content;
  will-change: transform; /* ← перманентно */
  padding-left: 24px;
}
```

Это противоречит собственному канону кодовой базы — arm/disarm-паттерну showcase (комментарий-обоснование: `components/sections/showcase.module.css:191-193` «постоянный слой — впустую память на дешёвых телефонах»). Коммит a7a0fc474 («will-change на время движения») доехал только до дорожек showcase.

## Target

`will-change: transform` ставится на дорожку в начале движения и снимается таймером 620мс после последнего движения. Целевой CSS — в обоих module.css строка `will-change: transform;` просто удаляется. JS — в `createStripEngine` (components/strip-engine.ts):

```ts
// target — внутри createStripEngine, рядом с прочим состоянием
let layerTimer: number | null = null;
const armLayers = () => {
  if (track.style.willChange !== "transform") {
    track.style.willChange = "transform";
  }
  if (layerTimer !== null) {
    clearTimeout(layerTimer);
  }
  layerTimer = window.setTimeout(() => {
    track.style.willChange = "";
    layerTimer = null;
  }, 620);
};
```

`armLayers()` вызывается: в начале drag-активации (обработчик pointerdown, рядом с `setPointerCapture`), при старте программного перехода (перед запуском rAF-петли твинa) и в обработчике wheel-шага. Таймер чистится в teardown движка, если есть.

## Repo conventions to follow

- Эталон: `components/sections/showcase.tsx:298-314` — `armLayers` с дедупом `el.style.willChange !== "transform"` и таймером LAYER_MS=620 (запас к 360/460мс переходам).
- Второй экземпляр: `components/sections/testimonials-carousel.tsx:404-423` (мобайл, 620мс) и `:704-723` (десктоп, 480мс).

## Steps

1. rentals-carousel.module.css:20 — удалить `will-change: transform;`.
2. tablet-strip.module.css:41 — удалить `will-change: transform;`.
3. strip-engine.ts — добавить armLayers (Target) и три точки вызова (pointerdown-активация, старт программного перехода, wheel-шаг); очистка таймера в teardown, если движок его имеет.
4. Убедиться, что ни один другой потребитель strip-engine не полагается на постоянный слой визуально (стеклянные плашки отзывов/тарифов едут детьми дорожки — промоушен дорожки на время движения, как у showcase, где стекло уже ездит на вооружаемой дорожке).

## Boundaries

- Do NOT трогать backdrop-filter/glass.
- Do NOT менять физику движка (флик/резинка/снап/длительности).
- Do NOT добавлять armLayers на карточки-дети — только дорожка.
- Если структура strip-engine.ts не совпадает с описанием (дриф от коммита) — STOP и доложить.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Слой-проба**: на стенде после 1с покоя `getComputedStyle(track).willChange === "auto"` для аренды и всех 5 планшетных полос; во время драга — `transform`; через 620мс после отпускания — снова `auto`.
- **Кадровая проба**: драг аренды на CPU×4 (375 и 768) — медиана/p95 не хуже базовых (медиана 8.3мс/120Гц, провалов 0).
- **Feel check**: первый драг после скролла к секции — без «проседания» первого кадра (арм происходит на pointerdown до первого движения).
- **Done when**: слой-проба и кадровая проба зелёные, визуал лент 1:1.
