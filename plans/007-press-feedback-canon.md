# 007 — Токен пресс-а и пресс-отклик дизайн-слоя: scale 0.97 на нажатии

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: HIGH
- **Category**: Physicality & origin (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 6 файлов (tokens.css, globals.css, button.tsx, icon-button.tsx, chip-button.tsx, list-row.tsx) + 2 точечных токен-фикса + DESIGN.md, ~60 строк

## Problem

Пресс-скейла нет нигде во фронте, а словарь «нажато» разъехался на пять
диалектов. Канон физичности (AUDIT.md §3): пресс = `transform: scale(0.97)`
на `:active` с переходом 100–160ms, тонко (0.95–0.98); «pressable elements
with no press feedback» — прямо в списке охоты. Продукт мобильный-first
(PWA): `hover` на телефоне не существует, и там, где отклика нет вообще,
нажатие не подтверждается ничем.

Три степени болезни:

1. **Ноль отклика** у самого массового нажимаемого элемента — ряда
   `ListRow` (вся история операций, пикеры, списки) и у `ChipButton`
   (чипы фильтров):

```tsx
// apps/frontend/shared/ui/design/list-row.tsx:82-85 — current
        'group/row flex w-full cursor-pointer items-center gap-3 px-6 py-2 text-left font-sans outline-none',
        'transition-colors focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
```

```tsx
// apps/frontend/shared/ui/design/chip-button.tsx:27-31 — current
        'inline-flex h-11 shrink-0 cursor-pointer items-center justify-center gap-1 rounded-pill bg-surface-muted px-5 font-sans text-sm font-medium text-content outline-none transition-colors',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'disabled:pointer-events-none disabled:opacity-50',
```

   (grep `active:` по list-row.tsx — ноль совпадений; в chip-button — тоже.)

2. **Только цвет** у главных примитивов — `Button` и `IconButton`
   (`transition-colors` исключает transform, скейла нет):

```tsx
// apps/frontend/shared/ui/design/button.tsx:31 — current
const primaryFill = 'bg-primary text-white hover:bg-primary-hover active:bg-primary-active';
// button.tsx:38 — база cva
  'inline-flex cursor-pointer items-center justify-center whitespace-nowrap font-sans font-medium transition-colors outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:pointer-events-none disabled:opacity-50',
```

3. **Токен-противоречие** — один и тот же фон `bg-surface-muted` в канве
   кнопок темнеет до отдельного `*-active`-токена, а в самодельных рядах
   остаётся hover-цветом:

```tsx
// apps/frontend/shared/ui/design/button.tsx:43 — current (канон: отдельный active-токен)
          'bg-surface-muted text-content hover:bg-surface-muted-hover active:bg-surface-muted-active',
// apps/frontend/widgets/property-detail/ui/PropertyDetailActions.tsx:77 — current (active == hover)
          className={`flex h-[52px] w-full cursor-pointer items-center rounded-button px-1 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-hover ${
// apps/frontend/shared/ui/design/icon-button.tsx:25 — current (secondary: active == hover)
          'text-content-tertiary hover:bg-surface-muted hover:text-content-secondary active:bg-surface-muted active:text-content-secondary',
```

Оговорка происхождения: цветовые hover/active-шаги кнопок выверены по
Figma-карточкам состояний, и пресс-скейл в макетах не нарисован. Но владелец
уже принял пресс-отклик как принцип — тап-пакет колеса (plans/001, DONE,
решение 14.10); этот план раскатывает тот же принцип на дизайн-слой.
Финальное слово — за владельцем при ревью тикета.

## Target

**Шаг 1 — токен длительности пресcа** в `apps/frontend/shared/styles/tokens.css`,
в блок единого движения сразу после `--dl-duration-move` (строка 103):

```css
  /* Пресс-отклик (аудит #1418, канон физичности): трансформ нажатия —
     150ms (бюджет кнопки 100–160ms), быстрее цветовых 250ms — система
     отвечает мгновенно, цвет догоняет. */
  --dl-duration-press: 150ms;
```

**Шаг 2 — класс-рецепт `.pressable`** в `app/globals.css`, в секцию
дизайн-слоя рядом с блоком `.hub-compact`/`.hub-compact-bar` (после строки
506, перед блоком «ПК-каркас»):

```css
/* Пресс-отклик интерактива дизайн-слоя (аудит #1418): scale(0.97) на
   нажатии — канон физичности (коридор 0.95–0.98). Трансформ едет на
   --dl-duration-press (150ms), цвета/прозрачность/кольцо — на своём
   --dl-duration-fast (250ms): нажатие подтверждается мгновенно, цвет
   догоняет. reduced-motion не трогает — 150ms уже короче порога, а
   фидбек пониманию сохраняем (стратегия §8). */
.pressable {
    transition:
        transform var(--dl-duration-press) var(--dl-ease),
        opacity var(--dl-duration-fast) var(--dl-ease),
        background-color var(--dl-duration-fast) var(--dl-ease),
        border-color var(--dl-duration-fast) var(--dl-ease),
        color var(--dl-duration-fast) var(--dl-ease),
        box-shadow var(--dl-duration-fast) var(--dl-ease);
}

.pressable:active {
    transform: scale(0.97);
}
```

Побочный честный эффект: `box-shadow` в рецепте означает, что
focus-visible-кольцо теперь тоже проявляется за 250ms, а не мгновенно —
считать приемлемым (кольцо мягче, не резче).

**Шаг 3 — применить класс и снять `transition-colors`** (иначе они воюют
за `transition-property`; класс без слоёв побеждает утилиту, но дубли
недопустимы):

```tsx
// button.tsx:38 — target (база cva)
  'pressable inline-flex cursor-pointer items-center justify-center whitespace-nowrap font-sans font-medium outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:pointer-events-none disabled:opacity-50',
```

```tsx
// icon-button.tsx:18 — target (база cva)
  'pressable inline-flex shrink-0 cursor-pointer items-center justify-center rounded-pill font-sans outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none disabled:opacity-50',
```

```tsx
// chip-button.tsx:27-31 — target (заодно появляется hover/active по канону Button secondary)
        'pressable inline-flex h-11 shrink-0 cursor-pointer items-center justify-center gap-1 rounded-pill bg-surface-muted px-5 font-sans text-sm font-medium text-content outline-none hover:bg-surface-muted-hover active:bg-surface-muted-active',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'disabled:pointer-events-none disabled:opacity-50',
```

```tsx
// list-row.tsx:82-85 — target
        'pressable group/row flex w-full cursor-pointer items-center gap-3 px-6 py-2 text-left font-sans outline-none',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
```

У `ListRow` внутренний `transition-colors` упомянут в title-спэне
(`group-hover/row:text-content-secondary`) — цвет заголовка теперь едет на
`color` из рецепта `.pressable` (переход унаследует? нет: transition живёт
на самом элементе с изменяющимся свойством — на спэне). Добавить на
title-спэн (`list-row.tsx:92`) `transition-colors` явно:

```tsx
// list-row.tsx:92 — target (спэн заголовка)
            'truncate text-base font-medium text-content transition-colors group-hover/row:text-content-secondary',
```

**Шаг 4 — токен-фиксы «active == hover»** (диалекты одного значения):

```tsx
// PropertyDetailActions.tsx:77 — target
          className={`flex h-[52px] w-full cursor-pointer items-center rounded-button px-1 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-active ${
```

```tsx
// icon-button.tsx:25 — target (secondary)
          'text-content-tertiary hover:bg-surface-muted hover:text-content-secondary active:bg-surface-muted-active active:text-content-secondary',
```

Проверить той же линзой все `active:`-строки фронтенда: каждое
`active:bg-*`/`active:text-*`, равное своему `hover:*`, меняется на
`*-active`-токен, если он существует (`grep -rn 'active:' --include='*.tsx'
apps/frontend | grep -v test`). Найдешь active-значение без токена —
СТОП и отчёт, не изобретать цвет.

**Шаг 5 — канон в DESIGN.md** (§8, рядом с записью о тап-пакете колеса):

```markdown
- **Пресс-отклик интерактива** (аудит #1418): нажатие подтверждается
  scale(0.97) на 150ms (`--dl-duration-press`) — класс `.pressable`
  дизайн-слоя на Button, IconButton, ChipButton, ListRow и рядах/карточках;
  цвета и кольцо едут параллельно на 250ms. Активные состояния — только на
  токенах `*-active` (никаких `active:* == hover:*`). Hover-затемнение
  остаётся цветовым; на тач-устройствах единственный отклик — скейл.
```

## Repo conventions to follow

- Кривая и длительности — только токены `--dl-*` (решение 26.08,
  `tokens.css:101-103`); новых кривых не вводить.
- Прецедент принципа — тап-пакет колеса: plans/001-wheel-tap-feedback.md
  (DONE, решение владельца 14.10).
- Обычные классы дизайн-слоя в globals.css без слоёв — прецедент
  `.hub-compact` (`globals.css:492`); они побеждают Tailwind-утилиты,
  поэтому `transition-colors` у носителей снимается, не сосуществует.
- Exemplar аккуратной cva-базы — `icon-button.tsx:17-19`.

## Steps

1. `tokens.css` — токен `--dl-duration-press` (шаг 1).
2. `globals.css` — класс `.pressable` (шаг 2).
3. `button.tsx`, `icon-button.tsx`, `chip-button.tsx`, `list-row.tsx` — по
   Target (шаг 3).
4. `PropertyDetailActions.tsx:77` и `icon-button.tsx:25` — токен-фиксы;
   затем grep-проход «active == hover» по правилу шага 4.
5. `DESIGN.md` — запись §8 (шаг 5).

## Boundaries

- Do NOT трогать `payments-favorites-screen.tsx` — Reorder-ряды живут на
  framer-трансформах, CSS-скейл конфликтует с drag (их случай — план 016).
- Do NOT трогать `tab-bar-row.tsx`/`user-button.tsx` (навигация — цветовой
  dialect по канону TabBar) и контролы выбора `radio/checkbox/
  round-checkbox` (у них есть цветовой active-фидбек — достаточно).
- Do NOT менять цветовые значения hover/active — только механизм пресcа и
  выравнивание active на существующие `*-active`-токены.
- Do NOT добавлять скейл внутрь `:active` у disabled — `pointer-events-none`
  уже исключает, отдельного правила не нужно.
- Если код в указанных местах не совпадает с цитатами (дрейф от коммита
  2d6925b76) — СТОП и отчёт, не импровизировать.

## Verification

- **Mechanical**: `cd apps/frontend && npx tsc --noEmit` — без ошибок;
  `npm run lint` — зелёный; `make frontend-test` — зелёный (у
  button.test.ts / round-action-button.test.ts — проверить, не тестируют ли
  класс-строки: если тестируют — обновить ожидания по Target, это не дрейф).
- **Feel check** (телефон + десктоп):
  - история операций: тап по ряду — строка мягко «вдавливается» (≈3%),
    отпускается на 150ms; на десктопе hover-затемнение не изменилось;
  - CTA-кнопка: нажатие — скейл + цветовой шаг одновременно, release
    мгновенный;
  - чип фильтров: до правки тап не подтверждался ничем — теперь скейл;
  - spam-тест: 10 быстрых тапов подряд — никакого дребезга, скейл
    ретаргетится (transition, не keyframes);
  - TabBar и переключатели — не изменились (вне скоупа);
  - DevTools → Rendering → prefers-reduced-motion: reduce — скейл
    сохраняется (фидбек), длительность 150ms.
- **Done when**: у Button/IconButton/ChipButton/ListRow пресс-скейл есть;
  «active == hover» в дизайн-слое не осталось (grep чист);
  `.pressable` задокументирован в DESIGN.md §8.
