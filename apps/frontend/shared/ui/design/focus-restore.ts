'use client';

import { useState } from 'react';

/** Безкольцевой возврат фокуса после закрытия модальных поверхностей
 * (залипшее :focus-visible-кольцо на триггере после «открыл модалку —
 * закрыл»; решение владельца 19.10 — контракт «никогда», включая
 * клавиатурное закрытие по Esc; сам фокус при этом возвращается —
 * Tab-навигация продолжается с места открытия, решение 2 того же
 * разговора).
 *
 * Механика бага: Radix Dialog/DropdownMenu на закрытии по умолчанию
 * делает triggerRef.focus() — программный фокус без опций, а эвристика
 * браузера :focus-visible (наследование клавиатурности от предыдущего
 * сфокусированного элемента и модальности последнего ввода) в части
 * сценариев матчит кольцо; у controlled-поверхностей без триггера Radix
 * фокус не возвращает вовсе. Примитивы пары capture/restore: capture
 * фиксирует хозяина (явный элемент — триггер в момент его события — или
 * document.activeElement на Content-поверхностях), restore гасит дефолт
 * Radix preventDefault'ом и ставит фокус с литералом focus({
 * focusVisible: false }), который браузер обязуется не матчить
 * (Chromium 145+, Firefox 104+, Safari 18.4+; на старых движках опция
 * игнорируется — осознанная деградация к прежнему поведению, решение
 * владельца 19.10). Потребители: Modal (modal.tsx, обе канвы — Radix
 * Dialog и vaul Drawer, автофокус-пропсы пробрасываются), SuccessPopup,
 * Menu (menu.tsx — захват на триггере, у DropdownMenu своего
 * onOpenAutoFocus нет). more-sheet не трогаем: Drawer с modal={false}
 * фокус на закрытие не возвращает — залипаться нечему.
 */

export type Focusable = {
  focus(options?: { readonly preventScroll?: boolean; readonly focusVisible?: boolean }): void;
  blur(): void;
};

/** Срез события Radix onOpenAutoFocus/onCloseAutoFocus (custom event). */
export type FocusAutoFocusEvent = {
  preventDefault: () => void;
};

/** Пара примитивов одного цикла «открыл — закрыл». */
export type FocusRestore = {
  /** Зафиксировать хозяина возврата: явный элемент (триггер в момент его
   * pointerdown/keydown — фокус на триггер браузер ставит позже) или
   * document.activeElement (Content-поверхности: Radix двигает фокус
   * внутрь после нашего хендлера). */
  readonly capture: (from?: Element | null) => void;
  /** Вернуть фокус хозяину без кольца: preventDefault гасит дефолт Radix
   * (именно его голый triggerRef.focus() и рисует залипшее кольцо). */
  readonly restore: (event: FocusAutoFocusEvent) => void;
};

export type FocusRestoreOptions = {
  /** Источник активного элемента для capture без аргумента; по умолчанию
   * — document.activeElement. Шов юнит-канона (vitest — node-env без
   * DOM). */
  readonly activeElement?: () => Element | null;
};

const isFocusableElement = (value: Element | null): value is Element & Focusable =>
  value !== null && typeof (value as unknown as Focusable).focus === 'function';

const focusWithoutRing = (target: (Element & Focusable) | null, active: () => Element | null): void => {
  if (target === null) return;
  // Хозяин может быть уже в фокусе: vaul гасит фокус-вход внутрь шита
  // (preventDefault открытого автофокуса) — триггер его не теряет, и
  // кольцо на таком элементе приезжает ретроактивно, сменой модальности
  // ввода на клавиатурную (trusted-замер Chromium 154: Escape по
  // клик-фокусу перематчивает :focus-visible без фокус-события). У
  // focus() по уже сфокусированному фокус-события нет — подавлению
  // опции не на чем сработать; blur даёт свежее событие, несущее
  // подавление.
  if (active() === target) {
    target.blur();
  }
  target.focus({ preventScroll: true, focusVisible: false });
};

export function createFocusRestore(options: FocusRestoreOptions = {}): FocusRestore {
  const activeElement = options.activeElement ?? (() => document.activeElement);
  let captured: (Element & Focusable) | null = null;
  return {
    capture: (from) => {
      const current = from ?? activeElement();
      captured = isFocusableElement(current) ? current : null;
    },
    restore: (event) => {
      event.preventDefault();
      focusWithoutRing(captured, activeElement);
      captured = null;
    },
  };
}

/** Хук-обвязка поверхностей дизайн-слоя: захват живёт от открытия до
 * закрытия через ре-рендеры (открыли → порисовались → закрыли), поэтому
 * инстанс — на ленивом useState, канонический способ стабильных
 * инстанс-полей. */
export function useFocusRestore(): FocusRestore {
  const [restore] = useState(() => createFocusRestore());
  return restore;
}

/** Пропсы Content-поверхности Radix (Dialog/Drawer): захват на открытии
 * (до того, как Radix уведёт фокус внутрь контента), безкольцевый
 * возврат на закрытии поверх дефолта composeEventHandlers. */
export function focusRestoreContentProps(restore: FocusRestore): {
  readonly onOpenAutoFocus: (event: FocusAutoFocusEvent) => void;
  readonly onCloseAutoFocus: (event: FocusAutoFocusEvent) => void;
} {
  return {
    onOpenAutoFocus: () => {
      restore.capture();
    },
    onCloseAutoFocus: (event) => {
      restore.restore(event);
    },
  };
}
