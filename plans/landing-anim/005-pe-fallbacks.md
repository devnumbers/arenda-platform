# 005 — PE-фолбэки: 92vh, hidden+clip, hero svh

- **Status**: DONE (реализация эволюционировала: каскад «vh; dvh» Lightning CSS
  вырезает из сборки по browserslist — фолбэки переведены на положительные
  `@supports`-блоки, которые минификатор сохраняет; проверено grep'ом собранного CSS.
  overflow-x аналогично: база hidden + `@supports (overflow-x: clip)` поверх)
- **Commit**: 613729c87
- **Severity**: LOW-MEDIUM
- **Category**: Cross-browser (ниже базлайна: «ничего не ломается»; философия — progressive enhancement, решение владельца 17.10)
- **Estimated scope**: 4 файла (app/globals.css, components/sections/hero.tsx, components/ui/modal.tsx, components/install-instructions.tsx)

## Problem

Единицы `dvh`/`svh` и значение `clip` без каскадных фолбэков — на движках старее их первой поддержки декларация отбрасывается целиком:

```tsx
// components/ui/modal.tsx:181 — current (и :213)
"relative flex max-h-[92dvh] w-full max-w-[520px] flex-col rounded-sheet bg-white font-sans outline-none",
// components/install-instructions.tsx:145 — current
"relative flex max-h-[92dvh] w-full max-w-[400px] flex-col gap-6 overflow-y-auto overscroll-y-contain rounded-[40px] bg-white p-6 font-sans outline-none",
```

На iOS 15.x (ниже базлайна 16.4) `max-height` отбрасывается → карточка в `fixed inset-0 flex items-center` вылезает за экран при длинном контенте, кнопки закрытия недостижимы.

```css
/* app/globals.css:244-247 — current */
html,
body {
  overflow-x: clip;
}
```

Ниже Safari 16.0 `clip` невалиден → горизонтальный пан от full-bleed «вылезаюк». Каскад `hidden` → `clip` сохраняет канон на поддерживающих (clip перебивает, скролл-контейнер не создаётся) — образец каскада уже в кодовой базе: `components/tablet-strip.module.css:20-21`.

```tsx
// components/sections/hero.tsx:29 — current (фрагмент)
className="relative mx-auto flex h-[calc(100svh-88px)] w-full max-w-[1920px] flex-col justify-start overflow-hidden rounded-[32px] p-8 pt-12 ... min-[1680px]:h-[1050px]"
```

```css
/* app/globals.css:296-300 — current */
html body[data-scroll-locked] {
  overflow: clip !important;
  margin-right: 0 !important;
  padding-right: 0 !important;
}
```

## Target

1. Каскады двух деклараций (фолбэк первой строкой, современное значение второй):
```css
html,
body {
  overflow-x: hidden;
  overflow-x: clip;
}
```
```css
/* @layer components, рядом с .dl-* фазами модалок */
.dl-max-h-viewport {
  max-height: 92vh;
  max-height: 92dvh;
}
```
```css
html body[data-scroll-locked] {
  overflow: hidden !important;
  overflow: clip !important;
  margin-right: 0 !important;
  padding-right: 0 !important;
}
```
2. Hero-высота — компонентным классом с тем же каскадом:
```css
/* @layer components */
.hero-panel-h {
  height: calc(100vh - 88px);
  height: calc(100svh - 88px);
}
```
В hero.tsx `h-[calc(100svh-88px)]` → `hero-panel-h`. Утилита `min-[1680px]:h-[1050px]` остаётся: utilities-слой идёт после components — на ≥1680 побеждает 1050, ниже работает класс.

## Repo conventions to follow

- Каскад-фолбэк: `components/tablet-strip.module.css:20-21` (`overflow: hidden; overflow: clip;`).
- Компонентные классы дизайн-слоя: `app/globals.css:185-206` (`.dl-overlay-in` и др.) — туда же положить `.dl-max-h-viewport`.

## Steps

1. globals.css:244-247 — добавить строку `overflow-x: hidden;` перед `overflow-x: clip;`.
2. globals.css @layer components — добавить `.dl-max-h-viewport` и `.hero-panel-h` (Target).
3. globals.css:296-300 — добавить `overflow: hidden !important;` первой строкой правила.
4. modal.tsx:181 и :213 — `max-h-[92dvh]` → `dl-max-h-viewport` (внутри cn: замена утилиты на кастомный класс, tailwind-merge его не дедуплицирует).
5. install-instructions.tsx:145 — то же.
6. hero.tsx:29 — `h-[calc(100svh-88px)]` → `hero-panel-h`.

## Boundaries

- Do NOT менять значения (92, 88, 1050) — только добавление фолбэк-деклараций.
- Do NOT трогать other dvh/svh использования вне перечисленных.
- Если строка не совпадает — STOP и доложить.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные; в собранном CSS для `.dl-max-h-viewport`/`.hero-panel-h` две декларации в правильном порядке (современное — последней), для html/body — hidden перед clip.
- **Функционально**: hero на 375/768/1440/1920 — высоты без изменений (пробойником сравнить offsetHeight до/после на всех ярусах); модалки/инсталл — размер без изменений; горизонтального пана нет на 375.
- **Done when**: собранный CSS содержит каскады; высоты hero построечно равны прежним на 4 ярусах.
