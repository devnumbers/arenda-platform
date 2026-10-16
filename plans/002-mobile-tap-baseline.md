# 002 — Мобильная база: инпут поиска 16px, сброс tap-highlight

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: HIGH (#1) / MEDIUM (#4)
- **Category**: Accessibility / mobile-native §2, §4
- **Estimated scope**: 2 файла

## Problem

1. `shared/ui/design/search-field.tsx:40` — `<input type="search">` с `text-sm`
   = 14px (токен `--text-sm: 0.875rem`, globals.css `@theme`). iOS Safari
   зумит страницу при фокусе в инпут <16px и не возвращает обратно. Гард-токен
   `--font-size-input: 1rem` объявлен в `shared/styles/tokens.css:59` именно
   для этого («maximumScale is intentionally omitted… the font-size guard is
   what prevents the zoom») и не потребляется нигде.
2. Сброса `-webkit-tap-highlight-color` нет во всём frontend: каждый тап по
   карточкам/рядам/заголовкам хаба на iOS даёт серю вспышку поверх
   `rounded-card`. Собственный `:active`-фидбек у компонентов есть
   (`active:opacity-80`), т.е. после сброса фидбек не теряется.

## Target

```css
/* app/globals.css — в базовый блок html/body */
html {
    -webkit-tap-highlight-color: transparent;
}
```

```tsx
/* search-field.tsx:40 — target */
className="min-w-0 flex-1 border-none bg-transparent text-[length:var(--font-size-input)] leading-[var(--line-height-input)] text-content outline-none placeholder:text-content-tertiary [&::-webkit-search-cancel-button]:hidden"
```

## Repo conventions

- Tailwind v4 произвольные значения через токен: `text-(--font-size-input)`
  или `text-[length:var(--font-size-input)]` — взять синтаксис, уже
  используемый в кодовой базе (проверить grep `text-\(`; если нет — форма
  `[length:var(--…)]`).
- Прецедент глобальной базы: `app/globals.css` (reduced-motion блок, `@theme`).

## Steps

1. `app/globals.css`: добавить `-webkit-tap-highlight-color: transparent` к `html`.
2. `search-field.tsx:40`: `text-sm leading-4` → размер/интерлайн от
   `--font-size-input`/`--line-height-input` (16/20). Высота поля `h-11` не меняется.

## Boundaries

- Не трогать `TextField`/`Textarea` (проверить их кегль; менять только если там тоже <16px).
- Никаких `user-scalable`/`maximum-scale` (запрещено — WCAG 1.4.4).

## Verification

- Механика: lint/build/test; computed font-size инпута = 16px в DevTools.
- Feel-check: iPhone — фокус в поиск платежей не зумит страницу; тап по карточке
  хаба без серой вспышки, нажатие отвечает `active:opacity-80`.
