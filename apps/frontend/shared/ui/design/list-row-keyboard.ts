import { createKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { runListboxAction } from '@/shared/ui/select/listbox-keyboard';

/**
 * Клавиатура ListRow одним швом (дизайн-слой, рядом с list-row.tsx):
 * кнопочный вариант — общая фабрика строк-кнопок createKeyboardActivation
 * (гвард #831 — keydown из вложенного сфокусированного элемента не
 * активирует строку и не давит её preventDefault; гвард #833 — авто-повтор
 * удержания глушится), option-вариант — общий listbox-модуль runListboxAction
 * (стрелки с wrap-around, Home/End, Enter/Space; Escape не гасится —
 * всплывает контейнеру списка). Неинтерактивная строка (без onSelect или
 * disabled) хендлер не навешивает.
 */
/** Срез keydown-события для клавиатуры ListRow: сверху совместим с
 * KeyboardActivationEvent (currentTarget сужен до HTMLElement, unknown его
 * принимает) и со структурным ListboxKeyboardEvent; React
 * KeyboardEvent<HTMLDivElement> удовлетворяет обоим. */
export type ListRowKeyboardEvent = {
  readonly key: string;
  readonly repeat: boolean;
  /** Элемент, на котором случился keydown: у keydown это сфокусированный
   * элемент — равен currentTarget, когда фокус на самой строке. */
  readonly target: unknown;
  /** Элемент-строка, на который навешан onKeyDown. */
  readonly currentTarget: HTMLElement;
  readonly preventDefault: () => void;
};

export type ListRowKeyboardOptions = {
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
  /** Роль строки (list-row.tsx): button — строка-кнопка (по умолчанию),
   * option — строка popup-списка внутри role=listbox. */
  readonly variant?: 'button' | 'option';
};

export function createListRowKeyboard<E extends ListRowKeyboardEvent>({
  onSelect,
  disabled = false,
  variant = 'button',
}: ListRowKeyboardOptions): {
  onKeyDown: ((event: E) => void) | undefined;
} {
  const interactive = onSelect !== undefined && !disabled;
  if (!interactive) {
    return { onKeyDown: undefined };
  }
  if (variant === 'option') {
    return {
      onKeyDown: (event: E): void => {
        // Возврат (новый сфокусированный option) строке не нужен — roving
        // focus ведёт контейнер списка, паритет прежнего inline-хендлера
        // list-row.tsx.
        runListboxAction(event, { onSelect });
      },
    };
  }
  // Кнопочный вариант ведёт себя как любая строка-кнопка дизайн-слоя: те же
  // гварды #831/#833 той же фабрикой.
  return createKeyboardActivation<E>({ onSelect, disabled });
}
