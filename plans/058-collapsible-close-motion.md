# 058 — CollapsibleSection: закрытие 300ms с fade-first выходом + фикс скачка паддинга

- **Status**: TODO
- **Commit**: 1328aa1f1
- **Severity**: HIGH
- **Category**: Easing & duration + Interruptibility (стыки модели плана 013; запрос владельца 27.10.2026 «закрытие секции Выполненные — топорное»)
- **Estimated scope**: 3 файла (globals.css, collapsible-section.tsx, DESIGN.md), ~30 строк

## Problem

Секция «Выполненные N» (обе ленты задач: «Все задачи» и задачи объекта) —
дизайн-компонент `CollapsibleSection` на Radix Collapsible. Модель движения
(grid-rows `0fr↔1fr` + opacity на transition, Presence-стражи — план 013)
правильная и не меняется. Грубость закрытия — в параметрах и стыках, четыре
дефекта:

**1. 24px-телепорт контента в первом кадре закрытия.** У триггера паддинг
`pb-6` появляется/исчезает мгновенно по смене `data-state` (баланс свёрнутой
карточки); `padding-bottom` ни в каком transition не участвует. В кадр
закрытия контент прыгает вниз на 24px (паддинг возникает НАД ним), потом
схлопывается. На открытии скачок невидим (контент ещё нулевой), на закрытии —
рывок старта:

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:42-49 — current
      <CollapsibleTrigger
        className={cn(
          'group flex w-full cursor-pointer items-center justify-between gap-3 px-6 pt-6 text-left font-sans outline-none',
          'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface rounded-card',
          // Свёрнутая карточка балансирует верхний паддинг нижним
          // (Figma 1532:50138: 0 0 24px), развёрнутой баланс даёт контент.
          'not-[[data-state=open]]:pb-6',
        )}
      >
```

**2. Закрытие 150ms на front-loaded кривой — «блик», а не движение.**
`--dl-ease` = `cubic-bezier(0.32, 0.72, 0, 1)` выносит ~87% пути к 75ms:
глаз видит «моргнула и схлопнулась». Канонные соседи: FAQ-аккордеон 300ms
(решение владельца 28.09, DESIGN.md §8), закрытие шита 330ms:

```css
/* apps/frontend/app/globals.css:429-467 — current (фрагмент) */
.collapsible-content[data-state='open'] {
    animation: collapsible-presence-in 200ms linear;
    grid-template-rows: 1fr;
    opacity: 1;
    transition:
        grid-template-rows 200ms var(--dl-ease),
        opacity 200ms var(--dl-ease);
}

@starting-style {
    .collapsible-content[data-state='open'] {
        grid-template-rows: 0fr;
        opacity: 0;
    }
}

.collapsible-content[data-state='closed'] {
    animation: collapsible-presence-out 150ms linear;
    grid-template-rows: 0fr;
    opacity: 0;
    transition:
        grid-template-rows 150ms var(--dl-ease),
        opacity 150ms var(--dl-ease);
}
```

**3. Opacity и grid-rows связаны одним темпом на выходе** — текст либо
мигает вместе с коробкой, либо режется поднимающейся линией обреза ещё
видимым. Рецепт: на закрытии opacity гаснет быстрее высоты — контент
растворяется, пока коробка полная; обрез едет по «пустому» пространству.

**4. Стрелка на чужой кривой и своём темпе** — `duration-200` на
Tailwind-дефолтной кривой (не `--dl-ease`), при закрытии доворачивается
после того, как контент уже схлопнулся:

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:55-61 — current
        <SmallArrowDown
          className={cn(
            'shrink-0 text-content-tertiary transition-transform duration-200',
            'group-data-[state=open]:rotate-180',
          )}
          aria-hidden
        />
```

Попутный дрейф документации в JSDoc компонента: анимация «через
`--radix-collapsible-content-height`» не существует со времён плана 013
(там была модель height-keyframes):

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:11-13 — current
 * изначально скрыты). Радикс даёт aria-expanded/aria-controls и
 * анимацию высоты через --radix-collapsible-content-height. */
```

Деталь, без которой план ломается: длительность Presence-стража закрытия
обязана равняться самой длинной ветке перехода. Страж — анимация кастомного
свойства без визуального эффекта; Radix держит контент в DOM до
`animationend` стража. Если закрытие удлинить до 300ms, а страж оставить
150ms — Radix размонтирует контент на середине перехода. Сегодня паритет
(150/150) — поэтому его просто сохраняют в новом масштабе (300/300).

## Target

**Шаг 1 — блок `.collapsible-content` в `app/globals.css`** (заменить
целиком блок с комментарием, строки 429-467):

```css
/* Сворачиваемая секция дизайн-слоя (#499; аудит #1418, план 013; тайминги
   27.10.2026 — решение владельца «закрытие топорное»): grid-rows 0fr→1fr
   + opacity на transition (канон #620, как tariff-faq) — без высотных
   keyframes: ре-клик посреди закрытия разворачивает секцию из текущей
   позиции. Тайминги — паритет FAQ-канона: 300ms на --dl-ease в обе
   стороны; на выходе opacity гаснет за 150ms — текст растворяется, пока
   коробка полная, обрез не режет видимый текст. Стражи Presence — по
   модели шита «Еще» (без визуального эффекта, имена in/out разные —
   Presence ждёт animationend); длительность стража = самой длинной ветке
   перехода, иначе Radix размонтирует контент на середине. */
@keyframes collapsible-presence-in {
    from { --collapsible-presence: 0; }
    to   { --collapsible-presence: 1; }
}

@keyframes collapsible-presence-out {
    from { --collapsible-presence: 0; }
    to   { --collapsible-presence: 1; }
}

.collapsible-content[data-state='open'] {
    animation: collapsible-presence-in 300ms linear;
    grid-template-rows: 1fr;
    opacity: 1;
    transition:
        grid-template-rows 300ms var(--dl-ease),
        opacity 300ms var(--dl-ease);
}

@starting-style {
    .collapsible-content[data-state='open'] {
        grid-template-rows: 0fr;
        opacity: 0;
    }
}

.collapsible-content[data-state='closed'] {
    animation: collapsible-presence-out 300ms linear;
    grid-template-rows: 0fr;
    opacity: 0;
    transition:
        grid-template-rows 300ms var(--dl-ease),
        opacity 150ms var(--dl-ease);
}
```

Ровно 300ms (не токен): у блока длительности литеральные с плана 013,
токена на 300ms нет (`--dl-duration-fast` 250ms, `--dl-duration-move`
350ms) — 300ms это паритет FAQ-канона.

**Шаг 2 — триггер и стрелка в `collapsible-section.tsx`**:

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:42-49 — target
      <CollapsibleTrigger
        className={cn(
          'group flex w-full cursor-pointer items-center justify-between gap-3 px-6 pt-6 text-left font-sans outline-none',
          'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface rounded-card',
          // Свёрнутая карточка балансирует верхний паддинг нижним
          // (Figma 1532:50138: 0 0 24px), развёрнутой баланс даёт контент.
          // Паддинг на transition: мгновенный свап двигал контент на 24px
          // в первом кадре закрытия.
          'not-[[data-state=open]]:pb-6 transition-[padding-bottom] duration-300 ease-apple',
        )}
      >
```

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:55-61 — target
        <SmallArrowDown
          className={cn(
            'shrink-0 text-content-tertiary transition-transform duration-300 ease-apple',
            'group-data-[state=open]:rotate-180',
          )}
          aria-hidden
        />
```

**Шаг 3 — JSDoc компонента** (строки 8-13, заменить последнюю фразу):

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:8-13 — target
/** Сворачиваемая секция дизайн-слоя (карта #497 — «Выполненные N» списка
 * задач, Figma 1531:12784): серая карточка с заголовком H3 и серым
 * счётчиком, справа — стрелка (вниз свёрнута, вверх развёрнута), контент —
 * строки списка. По умолчанию свёрнута (решение владельца: выполненные
 * изначально скрыты). Радикс даёт aria-expanded/aria-controls; движение —
 * transition grid-rows+opacity (.collapsible-content в globals.css):
 * 300ms --dl-ease в обе стороны, на закрытии opacity 150ms. */
```

**Шаг 4 — запись решения в DESIGN.md §8**: вставить пункт сразу после
буллета FAQ-аккордеона (заканчивается «…макеты 1879-70076/1929-76367.»,
строка 861) и перед буллетом «Кликабельность блоков — вся площадь»:

```markdown
- **Сворачиваемая секция дизайн-слоя — grid-rows 300ms, выход с fade-first**
  (`CollapsibleSection`, «Выполненные N» лент задач; техника — план 013,
  тайминги — решение владельца 27.10.2026): раскрытие и закрытие —
  transition `grid-template-rows: 0fr↔1fr` + opacity 300ms на `--dl-ease`
  (паритет FAQ-канона), на закрытии opacity гаснет за 150ms — текст
  растворяется, пока коробка полная, обрез не режет видимый текст.
  Балансировочный паддинг триггера (`pb-6` свёрнутой карточки) — на
  transition: мгновенный свап двигал контент на 24px в первом кадре
  закрытия. Стрелка и паддинг — те же 300ms `--dl-ease`; reduced-motion —
  канонные 150ms (блок globals.css). Стражи Presence держат контент в DOM
  до конца перехода (animationend) — секция прерываемая, ре-клик доезжает
  из текущей позиции.
```

## Repo conventions to follow

- Кривая и утилиты: `--dl-ease` = `cubic-bezier(0.32, 0.72, 0, 1)`
  (`shared/styles/tokens.css:102`), в Tailwind — `ease-apple`
  (`@theme`, globals.css:303). Жёсткие `Ns ease` запрещены (DESIGN.md §8).
- Прерываемая модель раскрытия — тариф-FAQ `tariff-faq.tsx:183-187`
  (grid-rows+opacity, контент всегда в DOM);Presence-стражи — модель
  `.more-sheet-*` (globals.css:408-435).
- Reduced-motion: не отключать, а укорачивать до ~150ms (решение владельца
  08.09, DESIGN.md §8) — существующий блок `app/globals.css:699-707` уже
  делает ровно это (`animation-duration`/`transition-duration: 150ms`
  перебивают шорткаты новых правил) — НЕ трогать.

## Steps

1. `apps/frontend/app/globals.css` — заменить блок `.collapsible-content`
   с комментарием (строки 429-467) на Target шага 1. Блок reduced-motion
   на строках 699-707 не трогать.
2. `apps/frontend/shared/ui/design/collapsible-section.tsx` — триггер,
   стрелка и JSDoc по Target шагов 2-3.
3. `apps/frontend/DESIGN.md` — буллет §8 по Target шага 4.
4. Проверить grep'ом, что других правил `.collapsible-content` в репо нет
   (кроме consumers-класса в tsx) — править нечего.

## Boundaries

- Do NOT трогать `tariff-faq.tsx` (его 300ms ease-out — решение владельца
  28.09) и `history-filters-sheet.tsx` (группы фильтров на 200/150 из
  плана 013 — паритизам отдельным словом владельца; скоуп 27.10 — только
  CollapsibleSection).
- Do NOT менять Radix-семантику (aria-expanded/aria-controls/hidden) и
  код потребителей (tasks-feed-screen, tasks-of-property-screen,
  task-section-card, tasks-skeletons, skeleton-section).
- Do NOT анимировать height/max-height — только grid-template-rows.
- Do NOT менять механизм Presence-стражей (кастомное свойство, разные
  имена in/out) — только их длительности (200→300, 150→300).
- Do NOT трогать reduced-блок globals.css:699-707.
- Модель прерываемости (transition + @starting-style + стражи) не
  меняется — только длительности, распределение opacity и стыки.
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: в `apps/frontend` — `npx tsc --noEmit` и `npm run lint`
  зелёные; `make frontend-test` зелёный; `make frontend-e2e
  TESTS='-g задач'` (Docker) зелёный — спек `e2e/task-edit-delete.spec.ts`
  работает с секцией «Выполненные» (авто-ожидания, 300ms не ломают).
- **Feel check** (dev-стенд, лента «Задачи», секция «Выполненные N»
  с 3+ строками):
  - закрытие: текст растворяется в первой половине движения, коробка
    спокойно доезжает; в первом кадре список НЕ прыгает вниз (старый
    рывок на 24px исчез);
  - открытие: 300ms плавно; стрелка доворачивается синхронно с коробкой,
    а не после неё;
  - быстрый двойной тап по заголовку — секция доезжает из текущей
    позиции, без провала в ноль (прерываемость жива);
  - DevTools → Animations, playback 10%: едут grid-template-rows,
    opacity и padding-bottom; height не анимируется;
  - DevTools → Elements в момент закрытия: контент остаётся в DOM до
    конца (~300ms), не рубится на середине;
  - Rendering → prefers-reduced-motion: reduce — оба направления и
    стражи на 150ms;
  - вьюпорт 320px: ничего не ломается, счётчик и стрелка на месте.
- **Done when**: в `.collapsible-content[data-state='closed']` переход
  300ms (rows) / 150ms (opacity), оба стража 300ms; у триггера есть
  `transition-[padding-bottom] duration-300 ease-apple`; стрелка на
  `duration-300 ease-apple`; JSDoc не упоминает
  `--radix-collapsible-content-height`; DESIGN.md §8 содержит новый
  буллет.
