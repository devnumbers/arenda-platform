import { describe, expect, it, vi } from 'vitest';
import { createListRowKeyboard } from './list-row-keyboard';

/**
 * Контракт клавиатуры ListRow на общем шве: кнопочный вариант — фабрика
 * строк-кнопок createKeyboardActivation с гвардами #831 (keydown из
 * вложенного сфокусированного элемента не активирует строку и не давит её
 * preventDefault) и #833 (авто-повтор удержания глушится), option-вариант —
 * делегация listbox-модулю (его интерналы запинены listbox-keyboard.test.ts,
 * здесь не дублируются). События — литералы-срезы {key, repeat, target,
 * currentTarget, preventDefault}, ни React, ни DOM (прецедент —
 * useKeyboardActivation.test.ts).
 */

type TestKeyDownEvent = {
  readonly key: string;
  readonly repeat: boolean;
  readonly target: unknown;
  readonly currentTarget: HTMLElement;
  readonly preventDefault: () => void;
};

const ACTIVATION_KEYS = ['Enter', ' '] as const;

/** Сама строка-див (currentTarget обработчика) и сфокусированная вложенная
 * кнопка («Переставить» в trailing-слоте витрины): keydown таргетится на
 * сфокусированный элемент; литералы — наглядные маркеры, не DOM
 * (useKeyboardActivation.test.ts), под HTMLElement — только для
 * currentTarget, который срез события сужает до него. */
const ROW = { element: 'row' } as unknown as HTMLElement;
const STAR = { element: 'star' };

const keyDown = (overrides: Partial<TestKeyDownEvent>): TestKeyDownEvent => ({
  key: 'Enter',
  repeat: false,
  target: ROW,
  currentTarget: ROW,
  preventDefault: () => undefined,
  ...overrides,
});

describe('createListRowKeyboard кнопочный вариант: паритет строк-кнопок', () => {
  it.each(ACTIVATION_KEYS)(
    'первое нажатие %j на самой строке — onSelect + preventDefault',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createListRowKeyboard<TestKeyDownEvent>({
        onSelect,
      });
      const prevented = vi.fn();

      onKeyDown?.(keyDown({ key, preventDefault: prevented }));

      expect(onSelect).toHaveBeenCalledOnce();
      expect(prevented).toHaveBeenCalledOnce();
    },
  );

  it('клавиша вне Enter/Space не активирует и не preventDefault', () => {
    const onSelect = vi.fn();
    const { onKeyDown } = createListRowKeyboard<TestKeyDownEvent>({
      onSelect,
    });
    const prevented = vi.fn();

    onKeyDown?.(keyDown({ key: 'ArrowDown', preventDefault: prevented }));

    expect(onSelect).not.toHaveBeenCalled();
    expect(prevented).not.toHaveBeenCalled();
  });

  it('неинтерактивная строка без onSelect — onKeyDown не навешивается', () => {
    const keyboard = createListRowKeyboard<TestKeyDownEvent>({});

    expect(keyboard.onKeyDown).toBeUndefined();
  });

  it('disabled=true — onKeyDown не навешивается', () => {
    const onSelect = vi.fn();
    const keyboard = createListRowKeyboard<TestKeyDownEvent>({
      onSelect,
      disabled: true,
    });

    expect(keyboard.onKeyDown).toBeUndefined();
  });
});

describe('createListRowKeyboard кнопочный вариант: keydown из вложенного интерактивного элемента (#831)', () => {
  it.each(ACTIVATION_KEYS)(
    'keydown %j сфокусированной вложенной кнопки (trailing «Переставить») не активирует строку и не preventDefault — нативная активация кнопки не давится',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createListRowKeyboard<TestKeyDownEvent>({
        onSelect,
      });
      const prevented = vi.fn();

      onKeyDown?.(keyDown({ key, target: STAR, preventDefault: prevented }));

      expect(onSelect).not.toHaveBeenCalled();
      expect(prevented).not.toHaveBeenCalled();
    },
  );
  // Контр-случай (target === currentTarget активирует) — «первое нажатие
  // %j на самой строке» в describe паритета: тот же литерал ROW.
});

describe('createListRowKeyboard кнопочный вариант: авто-повтор удержания (#833)', () => {
  it.each(ACTIVATION_KEYS)(
    'удержание %j активирует onSelect ровно один раз — авто-повторы глушатся',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createListRowKeyboard<TestKeyDownEvent>({
        onSelect,
      });

      onKeyDown?.(keyDown({ key }));
      for (let i = 0; i < 5; i += 1) {
        onKeyDown?.(keyDown({ key, repeat: true }));
      }

      expect(onSelect).toHaveBeenCalledOnce();
    },
  );

  it.each(ACTIVATION_KEYS)(
    'авто-повтор %j не активирует, но preventDefault остаётся (удержание не скроллит и не активирует нативно)',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createListRowKeyboard<TestKeyDownEvent>({
        onSelect,
      });
      const prevented = vi.fn();

      onKeyDown?.(keyDown({ key, repeat: true, preventDefault: prevented }));

      expect(onSelect).not.toHaveBeenCalled();
      expect(prevented).toHaveBeenCalledOnce();
    },
  );
});

describe('createListRowKeyboard option-вариант: делегация listbox-модулю', () => {
  it.each(ACTIVATION_KEYS)(
    '%j доходит до onSelect через runListboxAction — делегация с её preventDefault',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createListRowKeyboard<TestKeyDownEvent>({
        onSelect,
        variant: 'option',
      });
      const prevented = vi.fn();

      onKeyDown?.(keyDown({ key, preventDefault: prevented }));

      expect(onSelect).toHaveBeenCalledOnce();
      expect(prevented).toHaveBeenCalledOnce();
    },
  );

  it('неинтерактивная option без onSelect — onKeyDown не навешивается', () => {
    const keyboard = createListRowKeyboard<TestKeyDownEvent>({
      variant: 'option',
    });

    expect(keyboard.onKeyDown).toBeUndefined();
  });

  it('disabled=true у option — onKeyDown не навешивается', () => {
    const onSelect = vi.fn();
    const keyboard = createListRowKeyboard<TestKeyDownEvent>({
      onSelect,
      disabled: true,
      variant: 'option',
    });

    expect(keyboard.onKeyDown).toBeUndefined();
  });
});
