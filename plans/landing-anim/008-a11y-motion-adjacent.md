# 008 — Дешёвое a11y: копии showcase, FAQ aria-controls, Esc меню, noscript-видимость

- **Status**: DONE
- **Commit**: 613729c87
- **Severity**: LOW-MEDIUM
- **Category**: Accessibility (клавиатура/скринридер — дешёвая часть; клавиатурная навигация лент — отдельный тикет, решение владельца 17.10)
- **Estimated scope**: 4 файла (components/sections/showcase.tsx, components/sections/faq-list.tsx, components/mobile-menu.tsx, app/layout.tsx)

## Problem

1. Showcase рендерит 3 копии набора (COPIES=3, N=5): все 15 плашек — фокусируемые `<button>` без `aria-hidden`, слайды-копии 0/2 имеют `role="button"` и озвучиваются — 10 лишних tab-стопов, 30 «Показать: …» в скринридере:

```tsx
// components/sections/showcase.tsx:757-776 — current (плашки)
<button
  key={`${c}-${card.key}`}
  ref={(el) => { plateNodes.current[g] = el; }}
  type="button"
  aria-label={`Показать: ${card.name}`}
  className={s.plate}
  style={{ "--k": g === INITIAL ? 1 : 0 } as CSSProperties}
  onClick={() => selectNode(g)}
>
// :791-799 — current (слайды; tabIndex уже условный)
<div
  key={`${c}-${card.key}`}
  ref={(el) => { slideNodes.current[g] = el; }}
  data-slide-idx={g}
  role="button"
  tabIndex={near ? 0 : -1}
  aria-label={`Показать: ${card.name}`}
```

2. FAQ: у кнопки `aria-expanded`, но нет `aria-controls`, у ответа нет `id` (components/sections/faq-list.tsx:35-69).
3. Мобильное меню (кастомное, не Radix): Esc не закрывает — keydown-листенера нет (components/mobile-menu.tsx — весь файл).
4. До гидратации карусель showcase невидима (`visibility: mounted ? "visible" : "hidden"`, showcase.tsx:750); noscript-переопределение в layout.tsx:63-65 покрывает только `[data-reveal]` — no-JS пользователь видит пустоту.

## Target

1. Копии вне средней не фокусируемы и скрыты от SR:
```tsx
// плашки: aria-hidden={c !== 1} + tabIndex={c === 1 ? 0 : -1}
// слайды: aria-hidden={c !== 1} (tabIndex уже near ? 0 : -1, где near = c === 1)
```
2. FAQ: кнопке `aria-controls={`faq-panel-${groupIndex}-${itemIndex}`}`, блоку ответа `id` с тем же значением.
3. Esc-закрытие меню — в эффект лока (план 001, тот же файл: keydown Escape → startClose; если 001 откачен — отдельный эффект с deps [open]).
4. Карусель showcase получает `data-carousel-canvas`, noscript в layout.tsx дополняется правилом `[data-carousel-canvas]{visibility:visible!important}`.

## Repo conventions to follow

- aria-паттерны кодовой базы: `mobile-menu.tsx:136-138` (aria-expanded/controls/label на бургере), `testimonials-carousel.tsx:633` (`aria-hidden={q !== 0}` на невидимых слайдах).

## Steps

1. showcase.tsx: плашкам `aria-hidden={c !== 1}` и `tabIndex={c === 1 ? 0 : -1}`; слайдам `aria-hidden={c !== 1}`; div зоны (747-751) — атрибут `data-carousel-canvas`.
2. faq-list.tsx: id на блоке ответа (64-69) + aria-controls на кнопке (35-39).
3. mobile-menu.tsx: Esc — согласно плану 001 (шаг 2); если 001 в статусе отклонён — отдельный useEffect с keydown по [open].
4. app/layout.tsx:63-65 — в noscript-блок стиля добавить `[data-carousel-canvas]{visibility:visible!important}`.

## Boundaries

- Do NOT делать клавиатурную навигацию лент / focus-trap / фокус-фоллоу тарифной полосы — отдельный тикет (решение владельца 17.10).
- Do NOT трогать alt="" у фото внутри aria-label-кнопок (корректно: имя несёт label).
- Do NOT трогать vaul/модалки.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Клавиатура**: Tab по странице — плашки showcase встречаются ровно 5 раз; Enter на видимой плашке/слайде переключает карусель; Esc закрывает открытое меню.
- **Скринридер-смок**: (без устройства — по DOM) у 10 копий aria-hidden=true, у кнопок FAQ aria-controls указывает на существующий id.
- **No-JS**: отключить JS — карусель showcase видна.
- **Done when**: tab-стопов showcase ровно N (5), Esc работает, no-JS-секция видна, FAQ-связь в DOM.
