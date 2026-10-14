# 010 — Тосты: выход из текущей позиции + каскад reduced×mobile

- **Status**: TODO
- **Commit**: 2d6925b76
- **Severity**: HIGH
- **Category**: Interruptibility + Accessibility (аудит анимаций вне колеса, карта #1416 / тикет #1418)
- **Estimated scope**: 1 файл (`ToastProvider.module.css`), ~20 строк

## Problem

Тосты — каноническое «rapidly-triggered UI» (канон §4: «Anything triggered
rapidly … must use transitions or springs»), но их enter/exit — keyframes
через `cssTransition`. Полностью переводить их на transitions — большая
работа; точечно чинятся два конкретных дефекта.

**Дефект 1 — exit-кейфреймы стартуют из «полностью въехавшего» состояния.**
У обоих exit-кейфреймов явно прописан `from`, поэтому при вытеснении тоста,
не доигравшего вход (окно 350ms — штатный путь `showDisplacingToast`:
старый уходит exit-ом, новый одновременно въезжает), уходящая карточка
телепортируется из середины въезда (частичная прозрачность, 2/3 за экраном)
обратно в центр с полной непрозрачностью и только потом уезжает — рывок на
видном месте:

```css
/* apps/frontend/shared/ui/toast/ToastProvider.module.css:269-281 — current */
@keyframes toast-exit-right {
    from {
        opacity: 1;
        transform: translate3d(0, 0, 0);
    }

    to {
        opacity: 0;
        transform: translate3d(120%, 0, 0);
    }
}
```

(то же у `toast-exit-up`, строки ~287-299.) Лекарство каноничное: у CSS
keyframes без `from` стартовый кадр берётся из текущего вычисленного
состояния элемента — доигравший вход тост визуально не меняет (он и так в
центре с opacity 1), а недо-въехавший уходит из своей фактической позиции.

**Дефект 2 — на мобиле reduced-motion молча не работает.** Продукт
mobile-first: блок `@media (max-width: 767px)` стоит ВНИЗУ файла после
reduced-блока, та же специфичность, поздний источник побеждает — на
телефоне `animation-name` перебивается обратно на слайд:

```css
/* apps/frontend/shared/ui/toast/ToastProvider.module.css:328-350 — current,
   порядок блоков: reduced (328) … затем mobile (342) */
@media (prefers-reduced-motion: reduce) {
    .toastEnter {
        animation-duration: 150ms;
        animation-name: toast-fade-in;
        animation-timing-function: var(--dl-ease);
    }
    /* … .toastExit аналогично … */
}

@media (max-width: 767px) {
    .toastEnter {
        animation-name: toast-enter-down;
    }
    /* … .toastExit → toast-exit-up … */
}
```

Задокументированное поведение («reduced-motion — кроссфейд 150ms»,
DESIGN.md §8) на основной платформе не выполняется: reduced-пользователь
телефона получает слайд `translate3d(0, calc(-100% - 32px), 0)`.

## Target

```css
/* apps/frontend/shared/ui/toast/ToastProvider.module.css — target */
/* Дефект 1: from убран — exit стартует из текущего состояния элемента
   (canonical CSS: отсутствующий from = computed style на старте).
   Доигравший вход тост уходит как раньше; недо-въехавший — из своей
   фактической позиции, без телепорта в центр. */
@keyframes toast-exit-right {
    to {
        opacity: 0;
        transform: translate3d(120%, 0, 0);
    }
}

@keyframes toast-exit-up {
    to {
        opacity: 0;
        transform: translate3d(0, calc(-100% - 32px), 0);
    }
}
```

Порядок блоков — mobile ПЕРВЫМ, reduced ВТОРЫМ (reduced выигрывает каскад
на обоих носителях):

```css
/* Сначала платформа: */
@media (max-width: 767px) {
    .toastEnter {
        animation-name: toast-enter-down;
    }

    .toastExit {
        animation-name: toast-exit-up;
    }
}

/* Затем доступность — поверх платформы: */
@media (prefers-reduced-motion: reduce) {
    .toastEnter {
        animation-duration: 150ms;
        animation-name: toast-fade-in;
        animation-timing-function: var(--dl-ease);
    }

    .toastExit {
        animation-duration: 150ms;
        animation-name: toast-fade-out;
        animation-timing-function: var(--dl-ease);
    }
}
```

(Тела блоков не меняются — переносится целиком существующий CSS.)

## Repo conventions to follow

- Точки расширения — существующие блоки этого же файла; новые селекторы
  не вводить.
- Канон выходов — «exit mirrors enter» (комментарий над
  `.toastEnter`, строки 236-240) — сохраняется: меняется только стартовая
  точка, не траектория/кривая/длительность.
- Exemplar reduced-стратегии «укоротить, не отключить» — тот же reduced-
  блок (150ms кроссфейд).

## Steps

1. Убрать `from { … }` из `toast-exit-right` и `toast-exit-up`.
2. Перенести блок `@media (max-width: 767px)` ВЫШЕ
   `@media (prefers-reduced-motion: reduce)` (целиком, без правки тел).

## Boundaries

- Do NOT трогать `toast-enter-*`, `toast-fade-*`, `collapseDuration`,
  `--dl-duration-move`, кривые — только `from`-кадры и порядок блоков.
- Do NOT переводить тосты на `@starting-style`-модель (возможно позже;
  сейчас точечный фикс).
- Известное ограничение БЕЗ плана: drag-дисмисс тоста реагирует только на
  дистанцию (`draggablePercent={40}`, `ToastProvider.tsx:74`), без
  velocity — жест зашит в react-toastify v11 (`Math.abs(s.delta) >
  s.removalDistance`), конфига скорости у библиотеки нет. Считать
  документированным ограничением; возвращаться к вопросу, только если
  тосты станут первичной поверхностью.
- Если код не совпадает с цитатами — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit`, `npm run lint`, `make frontend-test`
  — зелёные (toast-gate.test.ts / show-displacing-toast.test.ts не должны
  поменяться — они про логику, не CSS).
- **Feel check**:
  - обычная замена тостов: старый уезжает вправо (или вверх на мобиле),
    новый въезжает — картинка не изменилась против коммита;
  - БЫСТРАЯ замена: вызвать два тоста подряд в пределах 350ms (например,
    два действия с тостом подряд) — уходящий больше НЕ прыгает в центр:
    уезжает из той позиции, где его застала замена;
  - телефон (≤767px): тост въезжает сверху/уходит вверх как раньше;
  - телефон + DevTools → Rendering → prefers-reduced-motion: reduce —
    тост теперь кроссфейд 150ms (не слайд) — фикс дефекта 2;
  - десктоп + reduced — кроссфейд 150ms (не регрессировал).
- **Done when**: у exit-кейфреймов нет `from`; mobile-блок стоит выше
  reduced-блока; оба feel-кейса воспроизводятся.
