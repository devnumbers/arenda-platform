'use client';

import { useEffect, useRef, useState } from 'react';

/**
 * Признак «виртуальная клавиатура открыта» (решение владельца 24.10.2026:
 * TabBar не едет с клавиатурой — пока она открыта, футер снят с экрана;
 * StickyBottomBar ведёт себя как прежде — resizes-content поднимает его
 * над клавиатурой, тикет #1421).
 *
 * Логика живёт в чистом слое: isEditableTarget классифицирует цель фокуса,
 * shouldKeyboardCloseOnResize решает закрывать ли состояние по выросшему
 * вьюпорту — события обычные аргументы, ни React-рефов, ни DOM (юнит-слой
 * тестируется в node; прецедент — createKeyboardActivation,
 * createLongPress). useVirtualKeyboardOpen — тонкая обёртка: focusin/
 * focusout на document + страховочный resize.
 *
 * Детектор — не геометрия, а фокус: focusin в редактируемый элемент —
 * детерминированный момент появления клавиатуры на coarse-pointer
 * устройствах (WebKit и Chromium там рисуют её на любой фокус editable).
 * Resize-страховка закрывает состояние, когда вьюпорт вырос обратно до
 * высоты на момент открытия — системные закрытия клавиатуры без blur
 * (кнопка «назад» Android, свайп), при которых focusout не приходит.
 * Гейт `(pointer: coarse)` — на ПК виртуальной клавиатуры нет, состояние
 * там не открывается вовсе (TabBar и так desktop:hidden). На сервере и в
 * первый клиентский рендер — false (в лад гидратации).
 */

/** Срез цели focus-события, достаточный классификатору: фабрика не знает
 * ни DOM-элементов, ни React-событий — хук подставляет нативную цель,
 * тесты — литералы. Наружу не торчит: isEditableTarget принимает unknown. */
type FocusTargetSlice = {
  readonly tagName?: unknown;
  readonly isContentEditable?: unknown;
};

/** Редактируемая цель фокуса — та, чей фокус поднимает виртуальную
 * клавиатуру: нативные INPUT/TEXTAREA и contenteditable-хосты. SELECT
 * и кнопки клавиатуру не поднимают. */
export function isEditableTarget(target: unknown): boolean {
  if (typeof target !== 'object' || target === null) {
    return false;
  }
  const slice = target as FocusTargetSlice;
  if (slice.isContentEditable === true) {
    return true;
  }
  return slice.tagName === 'INPUT' || slice.tagName === 'TEXTAREA';
}

/** Вьюпорт вернулся к высоте на момент открытия (или вырос) — клавиатура
 * закрылась без blur. Открытая клавиатура только сжимает вьюпорт
 * (resizes-content), поэтому «не ниже baseline» — честный признак
 * закрытия. */
export function shouldKeyboardCloseOnResize(heightAtOpen: number, heightNow: number): boolean {
  return heightNow >= heightAtOpen;
}

const COARSE_POINTER_QUERY = '(pointer: coarse)';

export function useVirtualKeyboardOpen(): boolean {
  const [open, setOpen] = useState(false);
  const heightAtOpenRef = useRef(Infinity);
  const closeTimerRef = useRef<number | null>(null);

  useEffect(() => {
    if (!window.matchMedia(COARSE_POINTER_QUERY).matches) {
      return;
    }
    const cancelPendingClose = (): void => {
      if (closeTimerRef.current !== null) {
        window.clearTimeout(closeTimerRef.current);
        closeTimerRef.current = null;
      }
    };
    const onFocusIn = (event: FocusEvent): void => {
      if (!isEditableTarget(event.target)) {
        return;
      }
      // Высота до анимации клавиатуры — baseline для resize-страховки;
      // при смене поля с уже открытой клавиатурой базой станет сжатый
      // вьюпорт — закрытие по возврату к нему всё равно честное.
      cancelPendingClose();
      heightAtOpenRef.current = window.innerHeight;
      setOpen(true);
    };
    const onFocusOut = (event: FocusEvent): void => {
      if (!isEditableTarget(event.target)) {
        return;
      }
      heightAtOpenRef.current = Infinity;
      // Фокус может в тот же жест перейти в соседнее поле: его focusin
      // придёт раньше таймера, поэтому закрытие откладывается до конца
      // тика — футер (и aboveTabBar-оффсет панели) не мигает между полями.
      cancelPendingClose();
      closeTimerRef.current = window.setTimeout(() => {
        closeTimerRef.current = null;
        setOpen(false);
      }, 0);
    };
    const onResize = (): void => {
      // Открыты, только пока вьюпорт не вырос обратно до baseline.
      setOpen((prev) => prev && !shouldKeyboardCloseOnResize(heightAtOpenRef.current, window.innerHeight));
    };
    document.addEventListener('focusin', onFocusIn);
    document.addEventListener('focusout', onFocusOut);
    window.addEventListener('resize', onResize);
    return () => {
      cancelPendingClose();
      document.removeEventListener('focusin', onFocusIn);
      document.removeEventListener('focusout', onFocusOut);
      window.removeEventListener('resize', onResize);
    };
  }, []);

  return open;
}
