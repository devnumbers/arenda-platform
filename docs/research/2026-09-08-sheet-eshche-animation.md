# Плавный выезд нижнего шита мобильного меню «Еще» (vaul + Next.js 16 / React 19 / Tailwind v4)

- Дата: 2026-09-08. Research по тикету [wayfinder-карты #556 «Единый хром экранов» — «Исследование: плавный выезд шита „Еще“» #557](https://github.com/devnumbers/arenda-platform/issues/557). Владелец: «выезд должен быть плавным и классным — это крайне важно».
- Контекст потребителя: `apps/frontend` — Next.js 16 (App Router), React 19, Tailwind v4, дизайн-слой `shared/ui/design` (ADR 0050: shadcn-паттерн поверх Radix), **vaul ^1.1.2 уже в проекте** и канонизирован (Modal: карточка ≥768 / vaul-шит <768); моушн-канон `--dl-ease cubic-bezier(0.32, 0.72, 0, 1)` — 250ms цвета, 350ms движение.
- Что строим: TabBar «Объекты / Уведомления / Еще» (мобайл/планшет ≤768); тап «Еще» → оверлей `rgba(23,26,28,0.5)` + белый лист (radius 40 сверху, drag-ручка 48×4) с двумя строками навигации и нижним рядом — сам TabBar (Figma 1721-64793, 1721-57140).
- Метод: web-research по первоисточникам — блог и код Эмиля Ковальски (vaul 1.1.2), официальные доки, GitHub-issues, доки shadcn/Base UI, MDN, NN/g.

## Резюме — что конкретно делать в нашем стеке

1. **Оставаемся на vaul ^1.1.2** (уже в `apps/frontend/package.json`). Его дефолтная кривая — ровно наш `--dl-ease`: `transform 0.5s cubic-bezier(0.32, 0.72, 0, 1)`, написана «to mimic iOS's Sheet». Вся drag-физика (momentum-закрытие по флику, дэмпинг, velocity-снап, масштабирование border-radius при протаскивании) встроена и написана автором, который специализируется именно на этом.
2. **Snap points НЕ использовать** — у нас односкоростной шит фиксированной высоты (~300–350px). Без `snapPoints` vaul работает в режиме «open/closed + drag-to-dismiss», что нам и нужно; snap points втянули бы измерение vh, `fadeFromIndex`, `repositionInputs` и лишние ветки кода.
3. **Двойной TabBar решается z-наложением + единым компонентом**: шит рендерится через `Drawer.Portal` в body с z-index выше TabBar; оверлей `rgba(23,26,28,0.5)` затемняет и настоящий TabBar, а нижний ряд шита — это тот же компонент TabBar внутри `DrawerFooter`. Гарантия отсутствия «прыжка» — идентичная внутренняя геометрия (строка 72px, `padding-bottom: env(safe-area-inset-bottom)`), а она бесплатно получается переиспользованием компонента. Скрывать реальный TabBar не обязательно — его перекрывает оверлей; при drag вниз «щель» под листом выглядит правильно (как в iOS).
4. **Тайминг под канон проекта**: лист — дефолт vaul (500ms) или 350–400ms через CSS-переопределение по `[data-vaul-drawer]` (vaul задаёт transition CSS-бейзлайном, а не только inline — обычный open/close переопределяется без `!important`); оверлей — 250ms fade (наш канон фонов), т.е. быстрее выезда. Закрытие — чуть быстрее открытия (250–300ms).
5. **`shouldScaleBackground` не включать**: для короткого нав-шита он даёт мало, а требует `vaul-drawer-wrapper` на корне layout и имеет известные баги (чёрные вспышки, issue #259). Наш тёмный оверлей 50% — достаточный и предсказуемый.
6. **Safe-area и PWA**: `viewport-fit=cover` в meta viewport + `padding-bottom: max(12px, env(safe-area-inset-bottom, 0px))` на футере шита и на TabBar — одинаково. Известный iOS-баг: `env()` может вернуть 0px в standalone-режиме (vercel/next.js discussion #81264) — держать фолбэк-отступ.
7. **«Повторный тап "Еще"» закрывает шит бесплатно**: пока шит открыт, настоящий TabBar под оверлеем — тап по «Еще» попадает в оверлей и закрывает шит (стандартный overlay-click Radix). Управление состоянием: `open={moreOpen}` + `onOpenChange` на `Drawer.Root`, контент шита статичен (не размонтируется по условию) — это же страхует от открытого бага #558 (пропажа анимации закрытия в controlled-режиме).
8. **`prefers-reduced-motion` гейтить вручную**: у vaul нет встроенной поддержки. По канону проекта не отключаем анимации полностью, а укорачиваем: в media-запросе reduce длительность листа до ~150–200ms и оставляем fade — drag-механика (touch) при этом не ломается.

## Рекомендованный рецепт реализации

### Шаг 1. Структура

- Один клиентский компонент `MoreSheet` (`"use client"`), состояние `open` живёт в компоненте TabBar'а.
- `Drawer.Root` без `snapPoints`, `shouldScaleBackground` не ставим; `dismissible` (default `true`) даёт закрытие по оверлею, Esc (Radix), свайп за ручку.
- Ре-триггер не используем (`Drawer.Trigger` не нужен) — открытие идёт из TabBar'а через controlled `open`.

```tsx
<Drawer.Root open={open} onOpenChange={setOpen}>
  <Drawer.Portal>
    <Drawer.Overlay className="fixed inset-0 z-50 bg-[rgba(23,26,28,0.5)]" />
    <Drawer.Content className="fixed inset-x-0 bottom-0 z-50 mx-auto max-w-md rounded-t-[40px] bg-white outline-none">
      <div className="mx-auto mt-2 h-1 w-12 rounded-full bg-black/20" aria-hidden /> {/* drag-ручка 48×4 */}
      <nav>…две строки по 3 пункта (иконка 24 + подпись 13/15)…</nav>
      <Drawer.Footer asChild>
        <TabBar onNavigate={() => setOpen(false)} />   {/* тот же компонент, что и на экране */}
      </Drawer.Footer>
    </Drawer.Content>
  </Drawer.Portal>
</Drawer.Root>
```

### Шаг 2. Длительности и кривые

Дефолт vaul 1.1.2: `transition: transform .5s cubic-bezier(.32,.72,0,1)` на `[data-vaul-drawer]` (CSS-бейзлайн, инжектируемый компонентом) + та же кривая на `[data-vaul-overlay]` fade. Под канон проекта:

```css
/* Tailwind v4, CSS-first: в @layer components */
[data-vaul-drawer]  { transition-duration: 400ms; }        /* выезд/задвиг */
[data-vaul-drawer][data-state='closed'] { transition-duration: 300ms; } /* закрытие быстрее */
[data-vaul-overlay] { transition-duration: 250ms; }        /* наш канон fade */
```

- CSS-бейзлайн vaul переопределяется атрибутным селектором той же специфичности в более позднем слое; **но** после drag-release vaul ставит transition **inline** (0.5s) — «флик-закрытие» останется на 500ms. Это ок: флик и должен закрывать быстро/по инерции.
- Почему не spring: Эмиль рекомендует springs для «живых» микро-взаимодействий и ease-out для snappy-UI, но для iOS-sheet-подобия его собственный ответ — именно эта кривая (из Ionic, «mimic iOS's Sheet»). Она совпадает с `--dl-ease`, так что spring-движок нам не нужен.
- Радиус 40px при drag vaul масштабирует сам (пропорционально прогрессу протаскивания) — «классность» из коробки, ничего не добавляем.

### Шаг 3. TabBar и «вырастание из футера»

- Нижний ряд шита = тот же React-компонент TabBar (props: активный пункт «Еще», `onNavigate` закрывает шит). Идентичность высоты строки (72px), иконок, подписей и safe-area-паддинга исключает прыжок при открытии/закрытии.
- Реальный TabBar остаётся на месте: оверлей (z выше) затемняет его. Вариант «прятать реальный бар» нужен только если дизайнер захочет, чтобы при открытии бар не затемнялся, а мгновенно «превращался» — тогда синхронно с `open` ставить `data-state` и `visibility:hidden` (без transition, чтобы не было гонки двух анимаций).
- При drag вниз шита под нижним краем листа виден затемнённый оверлеем настоящий бар — это каноничное iOS-поведение, отдельной работы не требует.

### Шаг 4. Safe-area, viewport, скролл-лок

- `<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">`.
- Футер шита и TabBar: `padding-bottom: max(12px, env(safe-area-inset-bottom, 0px))` — `max()` страхует от iOS-бага `env()=0` в standalone-режиме PWA.
- Скролл-лок тела — из коробки (Radix). Шит неконтентный (навигация), внутренний скролл не нужен → `touch-action: none` от vaul на `[data-vaul-drawer]` сработает без конфликтов. `overscroll-behavior` дополнительно не требуется, пока внутри нет скролл-областей.

### Шаг 5. Доступность

- role=dialog/aria-modal/focus-trap/Esc — даёт Radix под капотом vaul. Обязательно дать `Drawer.Title` (визуально скрытый «Ещё») — иначе Radix ругается в консоль.
- Повторный тап «Еще»: пока открыт шит, тап физически попадает в оверлей → закрытие. Т.е. требование «повторный тап закрывает» выполняется без кода; фокус после закрытия вернётся на триггер-область (если воспроизведётся баг #519 — `onOpenAutoFocus={(e) => e.preventDefault()}`).

## Сравнение альтернатив

| Вариант | Плюсы | Минусы | Вердикт |
|---|---|---|---|
| **vaul 1.1.2** (в проекте) | Полная drag-физика (velocity, momentum, damping, radius-scaling), Radix-a11y из коробки, дефолтная кривая = наш `--dl-ease`, уже канонизирован в `Modal` | Репозиторий официально «unmaintained» (последний релиз 14.12.2024); ряд открытых issues (scroll-lock, focus-restore) | **Остаёмся.** UI-примитив стабилен и заморожен; замена не оправдана ради одного шита |
| **Base UI Drawer** (`@base-ui/react`) | Активная разработка; shadcn/ui с июля 2026 по умолчанию на Base UI; CSS-driven анимации через `data-swiping`/`data-starting-style`, snap points, nested | Миграция всего проектного `Modal`; нет аналогов `shouldScaleBackground`/`handleOnly`/`repositionInputs`; drag-физика проще vaul | Кандидат на будущее, если vaul забагуется на React 19+; отдельное решение, не сейчас |
| **Radix Dialog + своя анимация** | Полный контроль | Писать руками velocity/momentum/damping — ровно та часть, что «крайне важна»; дорого и рискованно | Нет |
| **framer-motion (AnimatePresence + drag="y" + spring)** | Хорошая физика, `ReducedMotion="user"` из коробки | +зависимость (~30KB), a11y-dialog собирать самому, дублирует Radix | Нет |
| **Чистый CSS** | Дёшево | Нет drag-to-dismiss вообще — не проходит по требованию «честный свайп» | Нет |

Риск «unmaintained» смягчается тем, что библиотека — 18KB gzip без зависимостей поверх Radix: при форс-мажоре форк/патч тривиален (shadcn именно так и мигрировал на Base UI).

## Подводные камни и обходы

1. **Пропажа анимации закрытия в controlled-режиме** (issue #558, открыт, фев 2025): не класть контент шита под условный рендер, снимаемый синхронно с `onOpenChange(false)`. У нас контент статичен — риск только если начать рендерить пункты меню по данным.
2. **«Чёрный фон»/джанк от `shouldScaleBackground`** (issue #259, shadcn discussion #2905): не включаем; если когда-нибудь включим — нужен `[vaul-drawer-wrapper]` на корне layout и `setBackgroundColorOnScale={false}`.
3. **Inline transition после drag**: переопределение длительности CSS работает для обычного open/close (vaul 1.1.2 держит бейзлайн в CSS-правиле `[data-vaul-drawer]`), но post-drag закрытие получает inline `0.5s`. Обход не нужен (флик = инерция), либо `!important` — за счёт потери interruptibility, не советую.
4. **`env(safe-area-inset-bottom) = 0px` в iOS standalone PWA** (next.js discussion #81264): всегда фолбэк `max(Npx, env(...))` и проверка в `/ui-walkthrough` в standalone-режиме.
5. **Скролл-лок не высвобождается до unmount Root** (открытый issue) и **страничный скролл при закрытии на React 19+Tailwind** (issue #569): шит монтируем всегда (hidden по `open=false` — vaul/Radix сами размонтируют Portal-контент), а не создаем/убиваем `Drawer.Root`; если #569 воспроизведётся — держать один постоянный `Drawer.Root`, это же лечит фокус-восстановление (#519).
6. **Клавиатура**: `repositionInputs` актуален только при наличии инпутов; в нав-шите их нет; кейс «открыл шит при поднятой клавиатуре»: vaul слушает `visualViewport` с задержкой (признаёт сам автор). Обход — при открытии шита блюрить активный элемент.
7. **Композитинг**: внутри шита не анимировать ничего кроме `transform`/`opacity` (подтверждено Эмилем: только они живут на композиторе); `will-change: transform` на Content уже ставит сам vaul.
8. **Не оборачивать `Drawer.Content` в родителя с `transform`/`filter`** — fixed-позиционирование портального контента сломается (containing block), и drag сломается (известный класс issue «Dragging not possible when drawer isn't position: fixed»).
9. **`prefers-reduced-motion`**: vaul не гейтит — гейтим в CSS (укорачиваем до ~150–200ms, fade оставляем); drag-жесты не трогаем — это управление, а не декоративная анимация.

## Источники

- Emil Kowalski, «Building a drawer component» (emilkowal.ski/ui) — кривая `0.5s cubic-bezier(0.32,0.72,0,1)`, физика drag, visualViewport, нюанс CSS-переменных — https://emilkowal.ski/ui/building-a-drawer-component (2024)
- Emil Kowalski, «Great animations» — springs vs ease-out, <300ms, interruptibility — https://emilkowal.ski/ui/great-animations (2024)
- vaul docs: Getting Started / API / Snap Points — https://vaul.emilkowal.ski/getting-started, /api, /snap-points (2024)
- vaul 1.1.2 исходники (типы и дистрибутив: `closeThreshold @default 0.25`, `scrollLockTimeout @default 500ms`, `setBackgroundColorOnScale @default true`, CSS-бейзлайн `[data-vaul-drawer]`) — https://github.com/emilkowalski/vaul (последний релиз 14.12.2024; баннер «unmaintained»)
- Issue #259 «Why does shouldScaleBackground add a black background?» — https://github.com/emilkowalski/vaul/issues/259; shadcn/ui discussion #2905 — https://github.com/shadcn-ui/ui/discussions/2905
- Issue #558 «Lacking close animation when controlling via the open prop» (открыт 24.02.2025) — https://github.com/emilkowalski/vaul/issues/558
- Issue #519 (focus restore), #497 (focus trap), #569 (React 19 page scroll on close), «Body scroll lock is never released…» — https://github.com/emilkowalski/vaul/issues (2024–2025)
- shadcn/ui: Drawer на Base UI, «Migrating from Vaul» — https://ui.shadcn.com/docs/components/base/drawer; changelog «Base UI as the default» (июль 2026) — https://ui.shadcn.com/docs/changelog/2026-07-base-ui-default; discussion «Vaul is unmaintained» — https://github.com/shadcn-ui/ui/discussions/8982 (2025)
- Safe-area: MDN `env()` — https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/env; Polypane, «Using safe-area-inset to build mobile-safe layouts» — https://polypane.app/blog/using-safe-area-inset-to-build-mobile-safe-layouts/ (2024–2025); iOS standalone-баг — https://github.com/vercel/next.js/discussions/81264
- UX-канон паттерна: Nielsen Norman Group, «Bottom Sheets» — https://www.nngroup.com/articles/bottom-sheet/; Material Design, «Sheets: bottom» — https://m2.material.io/components/sheets-bottom; Smashing Magazine, «Bottom Navigation Pattern On Mobile Web Pages» — https://www.smashingmagazine.com/2019/08/bottom-navigation-pattern-mobile-web-pages/
- `prefers-reduced-motion`: MDN — https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@media/prefers-reduced-motion; motion.dev accessibility — https://motion.dev/docs/react-accessibility
