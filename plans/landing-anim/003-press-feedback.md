# 003 — Press-фидбек: быстрый active-фаз и мгновенный dim

- **Status**: DONE
- **Commit**: 613729c87
- **Severity**: MEDIUM
- **Category**: Physicality (нажатие отвечает на pointer-down мгновенно; бюджет 100-160мс)
- **Estimated scope**: 7 файлов, точечные правки классов

## Problem

Нажатия отвечают вяло или никак:
- Кнопки — цвет active доезжает за 200мс (бюджет нажатия 100-160мс, Apple: отклик на pointer-down):

```tsx
// components/button.tsx:16 — current
const BASE =
  "inline-flex cursor-pointer items-center justify-center rounded-2xl transition-colors duration-200 outline-none select-none focus-visible:ring-2 focus-visible:ring-inset";
```

- FAQ-строки — pressable вовсе без отклика до клика:

```tsx
// components/sections/faq-list.tsx:39 — current
className="flex w-full cursor-pointer items-center gap-4 text-left outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
```

- Кликабельные карточки отзывов — только `cursor-pointer`:

```tsx
// components/sections/testimonials-carousel.tsx:650 — current (мобайл, ±1)
className={`${className} cursor-pointer`}
// :820 — current (десктоп, interactive)
className={`${className} cursor-pointer`}
```

- Цветовые active-фазы с duration-200: `ui/icon-button.tsx:16`, `sections/contact.tsx:65` (чипы браузеров), `mobile-menu.tsx:139` (бургер), `sections/steps.tsx:68` (кнопки шагов), `install-instructions.tsx` (триггер-чипы, если имеют active:).
- Эталон в кодовой базе: `components/sections/showcase.module.css:337-339` — `.slide:active { opacity: 0.92; }` мгновенно.

## Target

1. Везде, где уже есть цветовой `active:*` и `transition-colors duration-200`, добавить `active:duration-75` — нажатие приезжает за 75мс, отпускание остаётся 200мс (правильная асимметрия: отклик быстрый, возврат мягкий).
2. Поверхностям без всякого отклика — мгновенный dim `active:opacity-80` (текстовые строки) или `active:opacity-[0.92]` (карточки-фото, паритет с showcase): opacity не входит в их transition-списки — срабатывает мгновенно.

## Repo conventions to follow

- Доска состояний кнопок 2864-5099 (комментарий button.tsx:3-8) — конечные точки hover/active НЕ менять, ускоряется только фаза нажатия.
- Мгновенный active на пунктах меню — exemplar `components/mobile-menu.tsx:183` (`active:bg-surface` без transition на фоне).

## Steps

1. `components/button.tsx:16` — BASE: `...transition-colors duration-200 active:duration-75 outline-none...`.
2. `components/ui/icon-button.tsx:16` — cva base: добавить `active:duration-75` после `duration-200`.
3. `components/sections/contact.tsx:65` — чипы: добавить `active:duration-75`.
4. `components/mobile-menu.tsx:139` — бургер: добавить `active:duration-75`.
5. `components/sections/steps.tsx:68` — кнопки шагов: добавить `active:duration-75`.
6. `components/sections/faq-list.tsx:39` — строка вопроса: добавить `active:opacity-80`.
7. `components/sections/testimonials-carousel.tsx:650` — `className={`${className} cursor-pointer active:opacity-[0.92]`}`; то же на :820 (десктоп) + там `active:duration-75` (у .slide-desk opacity в transition — иначе dim доедет за 440мс).
8. `components/sections/tariff-cards.tsx:279` — сегменты Год/Месяц: добавить `active:opacity-80`.
9. `components/site-header.tsx:44` и `components/site-footer.tsx:48,54` — текстовые ссылки: добавить `active:opacity-80` (временное состояние, не из доски — минимально-нейтральный dim).

## Boundaries

- Do NOT менять конечные точки цветов (hover/active значения из доски состояний).
- Do NOT трогать transition списков свойств (только duration через active-вариант).
- Do NOT трогать glass/showcase.
- Если строка в файле не совпадает с описанной — STOP и доложить.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Feel check**: на таче (375, DevTools device mode) нажатие каждой поверхности даёт видимый отклик до отпускания пальца; отпускание — мягкий возврат; у FAQ/карточек отзывов dim мгновенный (без анимации входа).
- Slow-mo (Animations panel 10%): нажатие кнопки — цвет active доезжает быстро, возврат — медленнее.
- **Done when**: все перечисленные поверхности дают отклик на нажатие; визуал покоя/hover 1:1 макетам.
