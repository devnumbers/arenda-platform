# 020 — TabBar: низ уважает safe-area (home indicator)

- **Status**: TODO
- **Commit**: 6ea8bc4ed
- **Severity**: HIGH
- **Category**: mobile-native §7 (safe areas) — фиксированный низ под home-indicator при включённом `viewport-fit=cover`; карта #1416 / тикет #1421
- **Estimated scope**: 1 файл, одна декларация + строка комментария

## Problem

`viewport-fit=cover` включён (`app/layout.tsx:59`), поэтому `env(safe-area-inset-bottom)`
на iOS standalone живой — и бар обязан им отступать. Фактически `<nav>` TabBar
(`shared/ui/design/tab-bar.tsx:100-103`, класс `:102`) прибит `bottom-0` без
всякого паддинга: на iPhone в standalone home-indicator наезжает на подписи
табов.

Комментарии при этом уверенно описывают противоположное — safe-area учтена:

- `tab-bar.tsx:25-26` — «Низ уважает safe-area (home indicator)»;
- `tab-bar.tsx:104-106` — «Фолбэк 12px в safe-area — тот же, что у шита
  "Еще": полоса и лист заканчиваются на одной высоте (research §4)»;
- `tab-bar-row.tsx:48-49` — «safe-area остаётся снаружи ряда (у бара —
  паддинг nav, у шита — паддинг листа)».

Вся остальная хром-цепочка уже считает высоту бара `72px + max(12px, env(safe-area-inset-bottom))`
и без фикса расходится с реальностью:

- StickyBottomBar над баром: `bottom-[calc(72px_+_max(12px,env(safe-area-inset-bottom)))]`
  (`sticky-bottom-bar.tsx:51`, комментарий `:65` — «safe-area уважает сам
  TabBar, под которым панель плавает») — панель висит на `12px + inset` выше
  фактического верха бара;
- лист «Еще» резервирует низ: `pb-[calc(72px+max(12px,env(safe-area-inset-bottom)))]`
  (`more-sheet.tsx:123`, комментарий `:40`) — на Android (inset = 0) лист
  резервирует 84px, бар занимает 72px: полоса и лист НЕ на одной высоте;
- запас контента под баром: `page-content.tsx:28` — тот же `max(12px, env)`;
- модалки: `pb-[max(1.5rem,env(safe-area-inset-bottom))]` (`modal.tsx:136`).

То есть фикс сводит код к уже записанному везде интенту: геометрия соседей
верна, врёт только сам бар.

## Target

```tsx
// apps/frontend/shared/ui/design/tab-bar.tsx:102 — target
<nav
  aria-label="Нижняя навигация"
  className="pointer-events-auto fixed inset-x-0 bottom-0 z-50 w-full bg-white pb-[max(12px,env(safe-area-inset-bottom))] font-sans desktop:hidden"
>
```

И дополнить внутренний комментарий бара (`:104-112`) одной мыслью, которой
там не хватает — где именно живёт safe-area:

```tsx
// …продолжение существующего комментария у TabBarRow:
// Паддинг nav — единственный носитель safe-area бара: ряд TabBarRow (72px)
// ничего о ней не знает и переиспользуется шитом «Еще». Фолбэк 12px в
// max() — тот же, что у листа и StickyBottomBar: все трое считают одну
// высоту 72px + max(12px, env), поэтому полоса, лист и панель сходятся
// на любой устройстве.
```

## Repo conventions to follow

- Канон нижнего inset — `max(12px, env(safe-area-inset-bottom))`:
  `more-sheet.tsx:123`, `sticky-bottom-bar.tsx:51,70`, `page-content.tsx:28`.
  Отдельный фолбэк в `env(x, 0px)` не нужен — 12px уже фолбэк.
- Высоту ряда (72px) и его классы не трогать: ряд общий с нижним рядом шита
  «Еще» (`tab-bar-row.tsx:48-49`), safe-area снаружи ряда — так и задумано.

## Steps

1. `apps/frontend/shared/ui/design/tab-bar.tsx`: добавить
   `pb-[max(12px,env(safe-area-inset-bottom))]` в класс `<nav>` (`:102`).
2. Там же дополнить комментарий (Target, п.2).

## Boundaries

- Do NOT трогать `tab-bar-row.tsx`, `more-sheet.tsx`, `sticky-bottom-bar.tsx`,
  `page-content.tsx` — их геометрия уже верна и после фикса сходится.
- Do NOT вешать safe-area на ряд TabBarRow — он общий с шитом.
- Do NOT менять высоту бара на десктопе — бар там скрыт (`desktop:hidden`).
- Если класс `<nav>` в `tab-bar.tsx:102` не совпадает с цитатой (дрейф от
  6ea8bc4ed) — СТОП и отчёт, не импровизировать.

## Verification

- **Mechanical**: `cd apps/frontend && npx tsc --noEmit` и `npm run lint` —
  зелёные; `make frontend-test` — зелёный; регресс
  `make frontend-e2e TESTS='-g frame-shell'` — зелёный (ассерты бара там —
  видимость/отсутствие, геометрию не замеряют).
- **Feel check** (телефон, протокол карты #1416; эмуляция feel не
  воспроизводит):
  - iPhone standalone: табы заканчиваются над home-indicator, белая полоса
    под ними до самого низа экрана;
  - iPhone: открыть шит «Еще» — низ листа и полоса бара на одной высоте;
    экран со StickyBottomBar — панель вплотную над баром, без щели;
  - Android (без inset): бар на 12px выше низа — согласован с листом «Еще»;
- **Done when**: класс на nav стоит; `make frontend-test` и регресс
  frame-shell зелёные.
