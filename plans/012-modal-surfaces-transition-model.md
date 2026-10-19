# 012 — Модальные поверхности: переход-модель @starting-style, токены таймингов, vaul 440/330, reduced 150ms

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: MEDIUM
- **Category**: Interruptibility + Cohesion & tokens + Accessibility (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 6 файлов (tokens.css, globals.css, modal.tsx, success-popup.tsx, DESIGN.md, ui-kit/navigation.tsx), ~120 строк

## Problem

Три поверхности-шита — три тайминговые системы и четыре пары значений,
ни одна не на токенах, и все keyframes-поверхности теряют mid-flight
разворот:

1. **Keyframes рестартуют** (канон §4): быстрое «закрыл → открыл» во время
   exit-анимации перезапускает вход с нуля — Radix держит контент
   смонтированным до конца exit, смена `animation-name` стартует с
   `from scale(0.95)` вместо разворота из текущей позиции:

```tsx
// apps/frontend/shared/ui/design/modal.tsx:158-159 — current (оверлей)
            'data-[state=open]:animate-[modal-overlay-in_350ms_var(--dl-ease)]',
            'data-[state=closed]:animate-[modal-overlay-out_250ms_var(--dl-ease)]',
// modal.tsx:172-173 — current (карточка)
              'data-[state=open]:animate-[modal-card-in_400ms_var(--dl-ease)]',
              'data-[state=closed]:animate-[modal-card-out_220ms_var(--dl-ease)]',
// apps/frontend/shared/ui/design/success-popup.tsx:45-46 — current (те же литералы)
              'data-[state=open]:animate-[modal-card-in_400ms_var(--dl-ease)]',
              'data-[state=closed]:animate-[modal-card-out_220ms_var(--dl-ease)]',
```

   Правильная модель уже есть в репо — шит «Еще» (решение 22.09,
   `globals.css:396-484`): transition + `@starting-style` + keyframes-стражи
   Presence на кастомном свойстве, с разворотом из недо-закрытой позиции.
   Модалки на неё не переведены.

2. **Один жест — три длительности**: открыть шит = 500ms (мобильная модалка,
   вендорный дефолт vaul: `node_modules/vaul/style.css:1-7`,
   `animation-duration: 0.5s`), 440ms (шит «Еще»), 400ms (карточка). Значения
   440/330 задокументированы («mimic iOS's Sheet», решение 22.09), но живут
   восемью копиями литералов; документация уже разъехалась с кодом
   (DESIGN.md:206 и :1819-1820 обещают «fade 250 / выезд 400 / закрытие 300»;
   комментарий `modal.tsx:27` — «вход шита 450ms»; витрина
   `app/ui-kit/design-layer/sections/navigation.tsx:249-250` — «выезд 400ms»).

3. **Reduced-motion дыры**: модальная карточка и SuccessPopup (scale-движение)
   не покрыты нигде; мобильная модалка получает полный слайд 500ms
   (у vaul в инжектируемом CSS reduced-ветки нет), тогда как стратегия §8 —
   «укоротить до ~150ms» — уже применена к тостам, history-row, more-sheet,
   screen-shell.

## Target

**Шаг 1 — токены модальных таймингов** в `shared/styles/tokens.css`, после
блока `--dl-duration-*` (после строки 103 и добавленного планом 007
`--dl-duration-press`):

```css
  /* Модальные поверхности (аудит #1418): пары вход/выход на поверхность.
     Шит — решение 22.09 «mimic iOS's Sheet» (440/330), карточка и оверлей —
     действующие значения модалки (400/220 и 350/250). Единый источник:
     правка тайминга поверхности = правка одного токена. */
  --dl-duration-sheet-in: 440ms;
  --dl-duration-sheet-out: 330ms;
  --dl-duration-modal-card-in: 400ms;
  --dl-duration-modal-card-out: 220ms;
  --dl-duration-modal-overlay-in: 350ms;
  --dl-duration-modal-overlay-out: 250ms;
  /* Ускоряющийся уход шита с экрана — вторая кривая дома (решение 22.09). */
  --dl-ease-exit: cubic-bezier(0.32, 0, 0.67, 0);
```

**Шаг 2 — карточка и оверлей модалки на переход-модель** (`app/globals.css`,
вместо ключей `modal-card-in/out` применения — кейфреймы движения заменить
парой «transition + @starting-style + стражи Presence», по образцу блока
`.more-sheet-*`):

```css
/* Модалка-карточка ≥768px: переход-модель шита «Еще» (решение 22.09),
   перенесённая на Radix Dialog (аудит #1418): закрытие разворачивается
   из текущей позиции, повторное открытие посреди exit доезжает, а не
   рестартует. Стражи Presence — без визуального эффекта, на кастомном
   свойстве, имена in/out разные — Presence ждёт animationend при смене
   data-state. */
@keyframes modal-presence-in {
    from { --modal-presence: 0; }
    to   { --modal-presence: 1; }
}

@keyframes modal-presence-out {
    from { --modal-presence: 0; }
    to   { --modal-presence: 1; }
}

.modal-overlay[data-state='open'] {
    animation: modal-presence-in var(--dl-duration-modal-overlay-in) linear;
    opacity: 1;
    transition: opacity var(--dl-duration-modal-overlay-in) var(--dl-ease);
}

@starting-style {
    .modal-overlay[data-state='open'] {
        opacity: 0;
    }
}

.modal-overlay[data-state='closed'] {
    animation: modal-presence-out var(--dl-duration-modal-overlay-out) linear;
    opacity: 0;
    transition: opacity var(--dl-duration-modal-overlay-out) var(--dl-ease);
}

.modal-card[data-state='open'] {
    animation: modal-presence-in var(--dl-duration-modal-card-in) linear;
    opacity: 1;
    transform: scale(1);
    transition:
        transform var(--dl-duration-modal-card-in) var(--dl-ease),
        opacity var(--dl-duration-modal-card-in) var(--dl-ease);
}

@starting-style {
    .modal-card[data-state='open'] {
        opacity: 0;
        transform: scale(0.95);
    }
}

.modal-card[data-state='closed'] {
    animation: modal-presence-out var(--dl-duration-modal-card-out) linear;
    opacity: 0;
    transform: scale(0.95);
    transition:
        transform var(--dl-duration-modal-card-out) var(--dl-ease),
        opacity var(--dl-duration-modal-card-out) var(--dl-ease);
}
```

Старые `@keyframes modal-card-in/out` и `modal-overlay-in/out` удалить,
 предварительно убедившись, что потребителей больше нет (после шага 3 —
grep `modal-card-in\|modal-overlay-in` по tsx — 0). Мёртвые
`modal-sheet-in/out` (`globals.css:319-327, 351-359`) удалить там же.

**Шаг 3 — классы вместо animate-утилит**:

```tsx
// apps/frontend/shared/ui/design/modal.tsx:157-159 — target (оверлей)
            'modal-overlay fixed inset-0 z-50 bg-overlay',
// (animate-[modal-overlay-…]-строки сняты — движением ведёт globals.css)

// modal.tsx:170-174 — target (карточка)
              'modal-card relative flex max-h-[92dvh] w-full max-w-[520px] flex-col rounded-sheet bg-white font-sans outline-none',
// (animate-[modal-card-…]-строки сняты)
```

```tsx
// apps/frontend/shared/ui/design/success-popup.tsx:42-47 — target
        <DialogOverlay className="fixed inset-0 z-50" />
        <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
          <DialogContent
            className={cn(
              'modal-card relative w-full max-w-[400px] rounded-sheet bg-white p-6 font-sans shadow-[0px_8px_24px_0px_rgba(0,0,0,0.12)] outline-none',
            )}
          >
```

(Оверлей SuccessPopup остаётся без `bg-overlay`/класса — он и сегодня
прозрачный; движением карточки ведёт `.modal-card`.)

**Шаг 4 — vaul мобильной модалки: 440/330 + reduced** (`app/globals.css`,
после блока `.more-sheet-*`):

```css
/* Мобильная модалка (vaul Drawer, modal.tsx <768px): вендорный дефолт 0.5s
   выровнен по шиту «Еще» (решение 22.09, токены шага 1). Специфичность
   0,2,0 бьёт базу vaul [data-vaul-drawer] 0,1,0 независимо от порядка
   бандла; правила .more-sheet-* (0,5,0) по-прежнему старше — шит «Еще»
   не задет. Drag не тронут: vaul двигает шит inline-стилями с
   transition:none, keyframes играют только на вход/выход без жеста. */
[data-vaul-drawer][data-state='open'] {
    animation-duration: var(--dl-duration-sheet-in);
}

[data-vaul-drawer][data-state='closed'] {
    animation-duration: var(--dl-duration-sheet-out);
}

@media (prefers-reduced-motion: reduce) {
    [data-vaul-drawer][data-state='open'],
    [data-vaul-drawer][data-state='closed'] {
        animation-duration: 150ms;
    }

    .modal-overlay[data-state='open'],
    .modal-overlay[data-state='closed'],
    .modal-card[data-state='open'],
    .modal-card[data-state='closed'] {
        animation-duration: 150ms;
        transition-duration: 150ms;
    }
}
```

**Шаг 5 — токенизировать литералы шита «Еще»**: в блоке `.more-sheet-*`
(`globals.css:450-483`) заменить все `440ms` → `var(--dl-duration-sheet-in)`,
`330ms` → `var(--dl-duration-sheet-out)`, `cubic-bezier(0.32, 0.72, 0, 1)`
на входе → `var(--dl-ease)`, `cubic-bezier(0.32, 0, 0.67, 0)` на выходе →
`var(--dl-ease-exit)` (8 копий литералов схлопываются в токены;
значения не меняются).

**Шаг 6 — доки в правду**:

- `apps/frontend/DESIGN.md:205-207`: «fade 250ms; выезд 400ms, закрытие
  300ms» → «выезд 440ms, закрытие 330ms (`--dl-duration-sheet-*`), оверлей
  гаснет синхронно листу».
- `apps/frontend/DESIGN.md:1819-1820`: «шит «Еще»: fade 250 / выезд 400 /
  закрытие 300» → «шит «Еще»: выезд 440 / закрытие 330
  (`--dl-duration-sheet-*`)».
- `apps/frontend/shared/ui/design/modal.tsx:26-29` (JSDoc): «вход шита
  450ms / карточки 400ms … выходы короче (280/220ms), фон 350/250ms» →
  «вход шита 440ms (`--dl-duration-sheet-in`), карточки 400ms, оверлей
  350ms; выходы 330/220/250ms; движение — переход-модель шита «Еще»
  (глобс-классы, решение 22.09)».
- `apps/frontend/app/ui-kit/design-layer/sections/navigation.tsx:249-250`:
  «Выезд 400ms на кривой vaul, оверлей — fade 250ms» → «Выезд 440ms
  (`--dl-duration-sheet-in`), оверлей гаснет синхронно; закрытие 330ms».

## Repo conventions to follow

- Эталон переход-модели — блок `.more-sheet-*` (`globals.css:396-484`):
  transition + `@starting-style` + стражи Presence на кастомном свойстве
  (обоснование в комментарии блока: «keyframes не разворачивались из
  текущей позиции»).
- Кривые/длительности — только токены (решение 26.08 + новые пары шага 1);
  exit-кривая шита — задокументированное решение 22.09, токенизируется,
  но не меняется.
- Тайминги 440/330 и 400/220/350/250 — не ре-литигируются (решения
  22.09/26.08); план сводит их к токенам и выравнивает вендорный дефолт.

## Steps

1. `tokens.css` — шесть duration-токенов + `--dl-ease-exit` (шаг 1).
2. `globals.css` — переход-модель `.modal-card`/`.modal-overlay`, снос
   старых кейфреймов (проверить grep), vaul-оверрайды + reduced (шаги 2, 4).
3. `modal.tsx`, `success-popup.tsx` — классы по Target (шаг 3).
4. `globals.css` — токенизация литералов `.more-sheet-*` (шаг 5).
5. Доки: DESIGN.md ×2, modal.tsx JSDoc, ui-kit/navigation.tsx (шаг 6).

## Boundaries

- Do NOT трогать push-back фона `.screen-shell` (решение владельца 14.10,
  `globals.css:566-577`) и его reduced-блок.
- Do NOT трогать drag-механику vaul (inline-стили, `transition:none` на
  жесте) и настройки VELOCITY — только keyframe-длительности.
- Do NOT трогать MoreSheet-компонент (`more-sheet.tsx`) — его модель
  эталон, меняются только литералы на токены.
- Do NOT менять значения таймингов (кроме выравнивания vaul 500→440/330).
- Radix Presence должен дождаться стража: длительность стража = длительности
  transition той же стороны — если feel-проверка покажет раннее
  размонтирование, СТОП и отчёт (не увеличивать стража вслепую).
- Если код не совпадает с цитатами (дрейф) — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные. `make frontend-e2e TESTS='-g модал|support-modal'` (Docker) —
  профильные спеки модалок зелёные.
- **Feel check** (десктоп ≥768 + телефон, по каждому: обычная модалка,
  SuccessPopup «Прочитать все» в уведомлениях, шит фильтров, модалка
  vaul <768):
  - вход/выход визуально идентичен коммиту (те же 400/220, 350/250, 440/330);
  - закрыть и МГНОВЕННО открыть снова: карточка разворачивается из
    недо-закрытой позиции, не рестартует с 0.95 (главный фикс);
  - мобильная модалка открывается за 440ms (было 500), reduced — 150ms
    кроссфейд-скейл;
  - шит «Еще» — не изменился ни на кадр (токены вместо литералов);
  - push-back фона — не изменился;
  - spam-тест: 10 открытий/закрытий подряд — без застреваний и миганий
    (стражи Presence отрабатывают);
  - поддержка drag: недо-свайпнутую модалку <768 можно дотянуть и
    отпустить — поведение vaul не сломано.
- **Done when**: grep `animate-[modal-` по tsx — 0; grep литералов
  `440ms|330ms` в блоке `.more-sheet-*` — 0; доки совпадают с токенами;
  feel-кейс «закрыл-открыл» чист на всех четырёх поверхностях.
