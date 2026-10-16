# 001 — Мобильный тап-пакет: отклик на тап по ряду пикера

- **Status**: TODO
- **Commit**: 997e03e97
- **Severity**: HIGH
- **Category**: Physicality & origin (Apple §1 — отклик на pointer-down; mobile-native §2, §5, §8)
- **Estimated scope**: 3 файла (globals.css, wheel-picker.tsx), ~15 строк

## Problem

Тап по ряду колеса — самое частое действие пикера — не отвечает разработанным
фидбеком:

1. `-webkit-tap-highlight-color` в проекте нигде не выключен → iOS Safari
   рисует свою серую вспышку по тапу (единственный видимый отклик), Android —
   свою. Это «самый громкий сигнал „это веб-сайт“» (mobile-native §2).
2. У рядов нет `:active`-фидбека (AUDIT §3: pressable elements with no press
   feedback). `apps/frontend/shared/ui/design/wheel-picker.tsx:118-134` —
   `WheelRow` рендерит `li` с `cursor-pointer`, но нажатие ничего не подсвечивает.
3. Ряды — текстовые контролы без `user-select:none` / `-webkit-touch-callout:none`
   → лонг-пресс на iOS выделяет текст «Октябрь» / показывает callout
   (mobile-native §8: «Long-press selects button text»).
4. Нет `touch-action: manipulation` → iOS Safari может задерживать `click`
   по ряду на double-tap-детект (mobile-native §5).

Текущий код ряда:

```tsx
// apps/frontend/shared/ui/design/wheel-picker.tsx:117-135 — current
  return (
    <li
      id={rowId}
      {...(option
        ? { role: 'option', 'aria-selected': selected }
        : { 'aria-hidden': true })}
      data-index={index}
      style={{ height: WHEEL_ROW_HEIGHT }}
      className={cn(
        // Единый кегль всех рядов (Figma 1539-82659): выбранный
        // темнеет, остальные серые — без каскада размеров.
        'flex shrink-0 cursor-pointer snap-center items-center justify-center font-sans text-xl leading-6 text-content-secondary transition-colors',
        dark && 'text-content',
      )}
    >
```

## Target

1. Глобальная база mobile-native в `@layer base` в
   `apps/frontend/app/globals.css` — вставить внутрь существующего блока
   `@layer base` сразу после правила для `a` (там же, где легаси-сброс
   `a { color: inherit; text-decoration: none; }`, перед закрывающей `}`
   слоя):

```css
    /* Мобильная нативность (канон mobile-native): серую вспышку тапа
       iOS/Android глушим глобально — отклик рисуют контролы через
       :active; контролам — мгновенный click без double-tap-задержки и
       неселектируемый текст (контрол — не контент). */
    html {
        -webkit-tap-highlight-color: transparent;
    }

    button,
    a,
    [role='button'] {
        touch-action: manipulation;
        user-select: none;
        -webkit-user-select: none;
    }
```

2. Ряды колеса: на `<ul>` в `wheel-picker.tsx` (классы не наследуются
   `touch-action`-ом, но `user-select:none` на контейнере виден детям через
   `auto`, а `touch-action` предков пересчитывается по цепочке) и `:active`
   на сам `li`:

```tsx
// apps/frontend/shared/ui/design/wheel-picker.tsx:117-135 — target
  return (
    <li
      id={rowId}
      {...(option
        ? { role: 'option', 'aria-selected': selected }
        : { 'aria-hidden': true })}
      data-index={index}
      style={{ height: WHEEL_ROW_HEIGHT }}
      className={cn(
        // Единый кегль всех рядов (Figma 1539-82659): выбранный
        // темнеет, остальные серые — без каскада размеров. active —
        // мгновенный отклик на тап по ряду (канон Apple §1): тот же
        // токен, что у вторичной Button.
        'flex shrink-0 cursor-pointer snap-center items-center justify-center font-sans text-xl leading-6 text-content-secondary transition-colors active:bg-surface-muted-active',
        dark && 'text-content',
      )}
    >
```

И на `<ul>` (wheel-picker.tsx:364-374) добавить к существующим классам
`select-none [-webkit-touch-callout:none] touch-manipulation` — в конец
строки className, перед `focus-visible:*`:

```tsx
        className="wheel-cascade relative flex h-[240px] list-none flex-col snap-y snap-mandatory overflow-y-scroll overscroll-y-contain py-[96px] outline-none [scrollbar-width:none] [overflow-anchor:none] select-none [-webkit-touch-callout:none] touch-manipulation focus-visible:ring-2 focus-visible:ring-primary [&::-webkit-scrollbar]:hidden"
```

(`-webkit-user-select` отдельно не нужен: Tailwind-класс `select-none`
ставит `user-select: none`, а дочерние `li` с `user-select: auto` вычисляют
`none` от родителя; Safari современных версий понимает свойство без префикса.)

## Repo conventions to follow

- Канон press-фидбека — фон, не scale: вторичная Button
  (`apps/frontend/shared/ui/design/button.tsx:43`):
  `bg-surface-muted text-content hover:bg-surface-muted-hover active:bg-surface-muted-active`.
  Тот же токен `bg-surface-muted-active` (#dfe0e2) на ряде даёт видимую
  подсветку поверх полосы выбора (`bg-surface-muted`, #f3f4f6).
- Все transition-утилиты слоя уже получают кривую `--dl-ease` и 150мс
  дефолтом (globals.css:301-303) — отдельный transition не нужен, класс
  `transition-colors` у ряда уже есть.
- Комментарии в CSS/TSX — по-русски, объясняют «почему», ссылаются на
  решения/канон (пример: блок «Мобильная нативность» выше; блок
  «Барабан WheelPicker» globals.css:1044).

## Steps

1. `apps/frontend/app/globals.css`: внутри `@layer base`, после правила
   `a { color: inherit; text-decoration: none; }` (и перед закрывающей `}`
   слоя), вставить блок «Мобильная нативность» из секции Target.
2. `apps/frontend/shared/ui/design/wheel-picker.tsx`: в `WheelRow` добавить
   `active:bg-surface-muted-active` к строке классов (см. Target) и обновить
   внутренний комментарий.
3. Там же: в `<ul>` добавить `select-none [-webkit-touch-callout:none]
   touch-manipulation` (см. Target).

## Boundaries

- Do NOT трогать `wheel-picker-sheet.tsx`, `month-year-picker.tsx`,
  `task-time-picker.tsx`, `Button` — кнопки шита покрываются глобальным
  блоком (`button`).
- Do NOT менять aria-атрибуты, разметку и геометрию (высоты, snap, py-[96px]).
- Do NOT добавлять зависимости и новые токены — `bg-surface-muted-active`
  уже существует.
- Если код в указанных местах не совпадает с цитатами (дрейф от коммита
  997e03e97) — СТОП и отчёт, не импровизировать.

## Verification

- **Mechanical**: `make frontend-test` — зелёный (CSS/классовые правки,
  логика не меняется); `cd apps/frontend && npx tsc --noEmit` — без ошибок.
- **Feel check** (мобильное устройство или DevTools device mode + реальный
  iPhone по сети):
  - тап по ряду — серая вспышка iOS исчезла; вместо неё ряд мгновенно
    подсвечивается фоном #dfe0e2 и гаснет за ~150мс после отпускания;
  - лонг-пресс на ряде (1.5с) — выделения текста и callout нет;
  - тап по кнопкам шита («Отменить»/«Выбрать») — вспышки нет, их родной
    `active:`-фон остался;
  - скролл колеса не сломан: флик инерция + снап работают
    (`touch-action: manipulation` сохраняет pan).
- **Done when**: вспышки нет на iOS/Android, ряд отвечает подсветкой на
  нажатие, лонг-пресс не выделяет текст, `make frontend-test` зелёный.
