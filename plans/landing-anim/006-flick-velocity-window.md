# 006 — Флик отзывов: оконная скорость ~100мс

- **Status**: DONE
- **Commit**: 613729c87
- **Severity**: LOW
- **Category**: Gesture physics (скорость по последнему pointermove шумная: «притормозивший» палец не даёт флик)
- **Estimated scope**: 1 файл (components/sections/testimonials-carousel.tsx, только мобильная ветка ≤480)

## Problem

Мобильная карусель отзывов считает скорость броска по последнему событию pointermove:

```ts
// components/sections/testimonials-carousel.tsx:526 — current
d.v = (event.clientX - d.lastX) / Math.max(1, event.timeStamp - d.lastT);
d.lastX = event.clientX;
d.lastT = event.timeStamp;
```

На таче последнее move перед отпусканием почти всегда стоит на месте (движения склеиваются в кадр) — флик может не сработать. Соседние движки решают это окном ~100мс:

```ts
// components/sections/showcase.tsx:650-659 — exemplar
const now = performance.now();
const windowStart = now - 100;
const last = s2.dragSamples.at(-1);
const ref = s2.dragSamples.find((smp) => smp.t >= windowStart);
if (last && ref && last.t - ref.t >= 0 && last.t >= windowStart) {
  const dt = Math.max(now - ref.t, 1);
  const vP = -(e.clientX - ref.x) / dt; // скорость p, px/мс
```

## Target

Хранить окно сэмплов (не более 6, как в showcase) и считать скорость по ~100мс окну на отпускании. Пороги не меняются (`MOB_FLICK_V`, `MOB_LONG_SWIPE_PX`).

```ts
// drag.current: вместо lastX/lastT/v — samples: { t: number; x: number }[]
// onPointerMove (после активации drag):
d.samples.push({ t: event.timeStamp, x: event.clientX });
if (d.samples.length > 6) {
  d.samples.shift();
}

// endDrag (вместо чтения d.v):
const last = d.samples.at(-1);
const ref = d.samples.find((smp) => smp.t >= event.timeStamp - 100);
const v = last && ref ? (last.x - ref.x) / Math.max(1, event.timeStamp - ref.t) : 0;
// далее проверки d.v <= -MOB_FLICK_V / d.v >= MOB_FLICK_V → v
```

## Repo conventions to follow

- Эталон: `components/sections/showcase.tsx:612,629-630,649-665` — `dragSamples` с капом 6 и оконной скоростью на release; там же комментарий-обоснование (движения склеиваются в кадр).

## Steps

1. testimonials-carousel.tsx: в структуре drag.current заменить `lastX/lastT/v` на `samples` (инициализация в onPointerDown пустым массивом + первый сэмпл точки старта).
2. onPointerMove: пушить сэмплы с капом 6 (Target), удалить строку 526 и инкремент lastX/lastT.
3. endDrag: вычислить `v` по окну 100мс (Target) и использовать в порогах флика (565-568).

## Boundaries

- Do NOT менять пороги (0.5px/ms, 160px) и логику long-swipe.
- Do NOT трогать планшетную/десктопную ветки и strip-engine (у него окно уже есть).
- Если строки дрифтовали — STOP и доложить.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Feel check** (375, тач): резкий короткий бросок с остановкой пальца в конце — карусель перелистывает; медленный свайп без скорости — не перелистывает (решение по расстоянию); вялый «недоброс» — отскок назад.
- **Done when**: бросок с «прилипшим» пальцем листит так же надёжно, как у showcase.
