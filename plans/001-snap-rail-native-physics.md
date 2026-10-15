# 001 — SnapRail: нативный тач + прокси мыши с физикой Apple, снос Embla

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: HIGH
- **Category**: физика жеста (apple-design §3–6, §9; mobile-native §9)
- **Estimated scope**: ~6 файлов frontend + package.json + e2e + DESIGN.md

## Problem

Embla 9.0.0-rc04 (3 RC-пакета) заменил нативный тач-скролл лент синтетическим
drag-движком: на iPhone физику UIScrollView имитирует JS вместо оригинала;
константы Apple (проекция/резинка/спринг) в опциях не настраиваются.
Предыдущее своё решение (94ba68fec) жаловалось на «топорность» (проекция
`0.8·v` вместо формулы Apple, экспоненциальный tween вместо спринта, жёсткий
упор в край без резинки у мыши).

## Target

Модуль `shared/ui/snap-rail` (публичный API `SnapRail {testId, className, children}`
не меняется), внутри — нативный скролл + pointer-прокси мыши/пера:

1. **Тач — 100% нативный** (гвард `pointerType: mouse|pen`): momentum, bounce
   и `scroll-snap-type: x mandatory` браузера. Никакого JS на горячем пути тача.
2. **Проекция остановки** (WWDC18 803): `D = scrollLeft + (v/1000)·(d/(1−d))`,
   `d = 0.998` (≈ `v·0.499`).
3. **Доезд** — критически демпфированный спринг в замкнутой форме
   (damping 1.0, response 0.4s, ω = 2π/response), velocity отпускания
   подаётся в начальные условия: `disp(t) = (d₀ + (ω·d₀ − v₀)·t)·e^(−ω·t)`,
   `x(t) = target + disp(t)`. Длительность = момент, когда |disp| ≤ 0.5px
   и |disp′| ≤ 5px/s (кап 2s). Чистая функция в `snap-rail-physics.ts` +
   `springEase(from, to, v0)` → кейфреймы `animate(from, to, {durationMs, ease})`.
4. **Резинка c=0.55** (переиспользовать `rubberBandOffset` из `pull-math.ts`):
   во время drag за край `scrollLeft` клампится в [0, max], визуальный вылет —
   `translateX(∓rubber(overshoot, clientWidth))` на контейнере (только пока
   жест за краем); на отпускании спринг возвращает трансформ в 0 одновременно
   с доездом scrollLeft; velocity-трекер считает RAW-позицию (не клампнутую).
   У тача резинка нативная, код не заходит.
5. **Сохранить доказанные механики** 94ba68fec: off-switch
   `scroll-snap-type:none` на жест, восстановление строго после доезда;
   click-гвард capture-фазой после реального drag; порог старта
   `isHorizontalDrag` (|dx|>|dy|, >8px); `setPointerCapture`; pointercancel —
   мгновенный возврат snap; wheel/новый pointerdown гасят доезд (микродоезд
   к ближайшему снапу); reduced-motion — без проекции, докат к БЛИЖАЙШЕМУ
   снапу 150мс (канон «укорачиваем»); RTL-гвард.
6. Классы ленты: `flex snap-x snap-mandatory gap-2 overflow-x-auto
   [scrollbar-width:none] [&::-webkit-scrollbar]:hidden
   [touch-action:pan-y_pinch-zoom]`; bleed-классы потребителя
   `max-tablet:-mx-6 max-tablet:px-6 max-tablet:[scroll-padding-inline:24px]`
   (scroll-padding вернуться — он и есть линия снапа натива).
7. Снос зависимостей: `embla-carousel`, `embla-carousel-react`,
   `embla-carousel-wheel-gestures` из package.json (+ lock).

## Repo conventions

- ADR 0031: математика — чистый модуль, жест — хук, движение — motion `animate()`.
- Прецедент velocity-трекера/резинки: `shared/ui/pull-to-refresh/pull-math.ts`.
- Старая реализация для сверки: `git show 94ba68fec:apps/frontend/shared/ui/drag-scroll/*`.

## Steps

1. TDD: `snap-rail-physics.test.ts` (red) → `snap-rail-physics.ts` (green):
   `projectStopPoint` (d=0.998), `pickSnapTarget` (как старый), `rubberBandOffset`
   (реэкспорт/делегирование pull-math), `criticallyDampedSpringEase` (форма
   кривой, velocity в t=0, инвариант остатка ≤0.5px, кап).
2. `use-snap-rail-drag.ts` из старого `use-drag-scroll.ts`: новая проекция/спринг,
   RAW-трекинг, резинка-трансформ, параллельный возврат трансформа и scrollLeft.
3. `snap-rail.tsx`: классы из п.6, прокси-хук; снести `snap-rail-options.ts`
   и его юниты (константы переезжают в physics-модуль).
4. Потребитель `SectionRail`: добавить `max-tablet:[scroll-padding-inline:24px]`.
5. package.json: минус три embla-пакета, `npm install`.
6. e2e `payments-hub-drag.spec.ts` переписать: `scrollLeft` вместо translate3d;
   снапы — зеркало `collectSnapPoints`; резинка — трансформ контейнера
   (сопротивление + возврат в 0 + край вровень); reduced-motion — различающий
   ассерт к ближайшему снапу; тач-тап жив (hasTouch); bleed 390.
7. DESIGN.md §6/§8: канон «нативный тач + прокси мыши, физика Apple — константы
   в коде»; /ui-kit витрина — текст раздела.

## Boundaries

- Не трогать тач-поведение натива; не добавлять зависимостей (framer-motion уже есть).
- Не менять публичный API SnapRail и bleed-рецепт потребителя (кроме scroll-padding).

## Verification

- Юниты physics зелёные; `npm run lint`, `npm run build`, `make frontend-test`.
- e2e drag-спека зелёная (desktop + 390 + hasTouch + reducedMotion).
- Feel-check на телефоне: тач = нативный скролл iOS (momentum, резинка, snap);
  мышью на десктопе: флик — доезд по спрингу без «топорности», у краёв вязнущая
  резинка и мягкий возврат; клик жив до 8px; wheel в подёте гасит доезд.
