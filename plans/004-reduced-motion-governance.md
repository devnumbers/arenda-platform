# 004 — Reduced-motion говернанс: модальные токены + системный укорачивающий блок

- **Status**: DONE (14.10.2026; механика 001 уточнена при приёмке: доезд — criticalSettle без перелёта, резинка справа — margin-расширение диапазона, слева — трансформ; детали в коде и DESIGN.md §8)
- **Commit**: efe10fd46
- **Severity**: MEDIUM
- **Category**: Accessibility (канон §8: «не отключаем, а укорачиваем до ~150ms»)
- **Estimated scope**: 3 файла

## Problem

Глобальный reduced-motion блок `app/globals.css:554-562` ловит только 4
селектора шита «Еще». Не покрыто:

- `shared/ui/design/modal.tsx:152-167` — keyframes-въезды модалки:
  overlay 350/250ms, карточка **400**/**220ms** — при reduced-motion едут
  полной длиной; длительности вкомпозированы литералами (400 вне шкалы
  250/350 вообще) — находка #9 когезии.
- `shared/ui/design/skeleton.tsx:17` — `animate-pulse` (Tailwind, 2s infinite)
  живёт вне токенной системы.

## Target

1. Длительности модалки — токены в `shared/styles/tokens.css`:

```css
/* Motion: модалки (ключевые въезды/выезды design-слоя). */
--motion-modal-overlay-in: 350ms;   /* = --dl-duration-move */
--motion-modal-overlay-out: 250ms;  /* = --dl-duration-fast */
--motion-modal-card-in: 400ms;
--motion-modal-card-out: 220ms;
```

Значения НЕ менять (400 остаётся 400 — смена длительности требует фил-чека
владельца; токенизация фиксирует значения в шкале).

2. `modal.tsx` — классы-хуки + токен-длительности:

```tsx
'data-[state=open]:animate-[modal-overlay-in_var(--motion-modal-overlay-in)_var(--dl-ease)]',
'data-[state=closed]:animate-[modal-overlay-out_var(--motion-modal-overlay-out)_var(--dl-ease)]',
/* карточка — аналогично; в cn() базовых строк добавить классы-хуки
   'modal-surface' (overlay + карточка), чтобы системный блок мог целиться */
```

3. Системный блок в `globals.css` (рядом с существующим, одним списком):

```css
@media (prefers-reduced-motion: reduce) {
    /* …4 селектора more-sheet… */
    .modal-surface[data-state='open'],
    .modal-surface[data-state='closed'] {
        animation-duration: 150ms;
    }
}
```

4. Скелетон-пульс: токен `--motion-pulse: 2s` в tokens.css + комментарий в
   `skeleton.tsx`: пульс — opacity-only, fade-семейство канона («fade
   остаётся»); бесконечный цикл укорачиванием не лечится (150мс = строб),
   поэтому under reduced-motion остаётся — решение зафиксировать словами
   канона в DESIGN.md §8 одной строкой.

## Steps

1. tokens.css: 4 модальных токена + `--motion-pulse`.
2. modal.tsx: токены в animate-[…], класс-хук `modal-surface` на overlay и карточку.
3. globals.css: расширить media-блок селекторами `.modal-surface[data-state=…]`.
4. skeleton.tsx: комментарий канона (можно без замены animate-pulse — токен
   `--motion-pulse` документирует значение; допустимо оставить Tailwind-анимацию
   с комментарием-привязкой).

## Boundaries

- Не менять сами длительности (кроме 150мс под reduced-motion).
- Не трогать more-sheet-селекторы и vaul.

## Verification

- DevTools Rendering → emulate prefers-reduced-motion: попап успеха «Избранных»
  въезжает за 150мс; без эмуляции — 400мс как раньше.
- lint/build/frontend-test.
