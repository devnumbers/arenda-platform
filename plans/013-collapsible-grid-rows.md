# 013 — CollapsibleSection: grid-rows transition вместо высотных keyframes; раскрытие групп в шите фильтров

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Interruptibility + Performance + Missed opportunities (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 3 файла (globals.css, collapsible-section.tsx, history-filters-sheet.tsx), ~60 строк

## Problem

**Часть 1 — CollapsibleSection** (секции «Выполненные N» лент задач):
раскрытие анимируется keyframes по явному `height` — это и layout-анимация
(канон §5: height = layout+paint+composite каждый кадр), и рестарт посреди
движения (канон §4): ре-клик в 150ms окно закрытия перезапускает
`collapsible-up` с `height: var(…)` — полураскрытый контент проваливается
в ноль и разворачивается заново. Стрелка при этом на transition-transform и
ретаргетится, контент — нет:

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:65 — current
          'overflow-hidden data-[state=open]:animate-[collapsible-down_200ms_var(--dl-ease)] data-[state=closed]:animate-[collapsible-up_150ms_var(--dl-ease)]',
```

```css
/* apps/frontend/app/globals.css:375-393 — current */
@keyframes collapsible-down {
    from { height: 0; }
    to { height: var(--radix-collapsible-content-height); }
}

@keyframes collapsible-up {
    from { height: var(--radix-collapsible-content-height); }
    to { height: 0; }
}
```

**Часть 2 — шит фильтров истории**: группы фильтров раскрываются телепортом
(`{expanded && …}` — контент появляется/исчезает мгновенно, содержимое ниже
прыгает), шеврон жёстко свапается Up↔Down без проворота:

```tsx
// apps/frontend/widgets/history/ui/history-filters-sheet.tsx:611-618 — current
          {expanded ? (
            <SmallArrowUp className="ml-1 h-6 w-6 shrink-0" aria-hidden />
          ) : (
            <SmallArrowDown className="ml-1 h-6 w-6 shrink-0" aria-hidden />
          )}
        </button>
      </div>
      {expanded && (
        <div className="pb-1">
```

Канон раскрытия в репо уже принят дважды: `grid-template-rows: 0fr→1fr` +
opacity (решение #620, работает в tariff-faq) — часть 1 подводит
дизайн-слой к той же технике; часть 2 — добавка категории missed
opportunities: «изменения состояния, которые телепортируются», фикс
копирует готовый канон.

## Target

**Шаг 1 — переход-модель секции** (`app/globals.css` вместо
`collapsible-down/up`):

```css
/* Сворачиваемая секция дизайн-слоя (#499; аудит #1418): grid-rows 0fr→1fr
   + opacity на transition (канон #620, как tariff-faq) — без высотных
   keyframes: ре-клик посреди закрытия разворачивает секцию из текущей
   позиции. Стражи Presence — по модели шита «Еще» (без визуального
   эффекта, имена in/out разные — Presence ждёт animationend). */
@keyframes collapsible-presence-in {
    from { --collapsible-presence: 0; }
    to   { --collapsible-presence: 1; }
}

@keyframes collapsible-presence-out {
    from { --collapsible-presence: 0; }
    to   { --collapsible-presence: 1; }
}

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

(Поведение на первой отрисовке не меняется: сегодня keyframes тоже играют
на маунте уже-открытой секции.)

**Шаг 2 — класс вместо animate-утилит**:

```tsx
// apps/frontend/shared/ui/design/collapsible-section.tsx:65 — target
          'collapsible-content overflow-hidden',
```

(Radix ставит data-state; hidden-семантика Radix не меняется.)

**Шаг 3 — reduced-motion** — в существующий reduced-контекст
(`app/globals.css`, рядом с блоком плана 012):

```css
@media (prefers-reduced-motion: reduce) {
    .collapsible-content[data-state='open'],
    .collapsible-content[data-state='closed'] {
        animation-duration: 150ms;
        transition-duration: 150ms;
    }
}
```

**Шаг 4 — группы фильтров**: паттерн tariff-faq (контент всегда в DOM,
`inert` держит свёрнутое вне tab-порядка), значения канона дизайн-слоя
200/150:

```tsx
// apps/frontend/widgets/history/ui/history-filters-sheet.tsx:611-618 — target (шеврон)
          <SmallArrowDown
            className={cn(
              'ml-1 h-6 w-6 shrink-0 text-content-tertiary transition-transform duration-200 ease-apple',
              expanded && 'rotate-180',
            )}
            aria-hidden
          />
```

```tsx
// apps/frontend/widgets/history/ui/history-filters-sheet.tsx:618+ — target (контент)
      <div
        inert={!expanded}
        className={cn(
          'grid transition-[grid-template-rows,opacity] ease-apple',
          expanded ? 'opacity-100 [grid-template-rows:1fr] duration-200' : 'opacity-0 [grid-template-rows:0fr] duration-150',
        )}
      >
        <div className="overflow-hidden pb-1">
          {/* … прежнее содержимое без изменений … */}
        </div>
      </div>
```

(Стрелки `SmallArrowUp` больше не нужны в этом месте — импорт почистить,
если нигде больше не используется.)

## Repo conventions to follow

- Техника 0fr→1fr — решение #620, exemplar `tariff-faq.tsx:183-187`
  (внутренний div режет переполнение, контент всегда в DOM, `inert` для
  свёрнутого).
- Стражи Presence на кастомном свойстве — модель `.more-sheet-*`
  (`globals.css:408-435`), причина задокументирована в комментарии блока.
- Тайминги 200/150 секции — действующие значения CollapsibleSection, не
  меняются; кривая только `--dl-ease`/`ease-apple`.

## Steps

1. `globals.css` — переход-модель `.collapsible-content` + reduced (шаги 1, 3).
2. `collapsible-section.tsx:65` — класс по Target (шаг 2).
3. `history-filters-sheet.tsx` — шеврон и контент по Target (шаг 4).
4. Grep: потребители CollapsibleSection (tasks-feed-screen.tsx,
   tasks-of-property-screen.tsx и др.) — код самих потребителей не меняется.

## Boundaries

- Do NOT трогать tariff-faq (его 300ms ease-out — задокументированное
  решение владельца 28.09; у него не хватает только reduced — этот пробел
  закрывает план 014).
- Do NOT менять Radix-семантику (aria, hidden) CollapsibleSection.
- Do NOT анимировать высоту напрямую (никаких height/max-height) — только
  grid-template-rows.
- Секция с изначально открытой секцией на странице: первый рендер
  анимирован так же, как сегодня (keyframes на маунте) — это норма, не
  дрейф.
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные; `make frontend-e2e TESTS='-g задач'` — ленты задач зелёные.
- **Feel check**:
  - лента задач: «Выполненные N» раскрывается/сворачивается плавно (200/150),
    визуально как раньше;
  - быстрый двойной тап по заголовку секции — секция разворачивается из
    полузакрытой позиции, без «провала в ноль» (главный фикс);
  - раскладка соседей едет синхронно раскрытию, без прыжка в конце;
  - шит фильтров: тап по группе — контент разворачивается из шапки-триггера
    (200ms), шеврон проворачивается (не свапается), содержимое ниже
    сдвигается плавно; свёрнутая группа не доступна по Tab (inert);
  - DevTools → Rendering → prefers-reduced-motion: reduce — оба раскрытия
    150ms;
  - DevTools → Animations 10%: движение строки — grid-rows+opacity,
    height не анимируется.
- **Done when**: `@keyframes collapsible-down|up` удалены; двойной тап по
  секции не рестартует анимацию; группы фильтров не телепортируются.
