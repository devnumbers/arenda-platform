import { describe, expect, it, vi } from 'vitest';
import { createKeyboardActivation } from './useKeyboardActivation';

/**
 * Контракт клавиатурной активации строк-кнопок (тикеты #833, #831). Хук —
 * тонкая обёртка без React-состояния, поведенческие гарантии пинятся на
 * чистой фабрике createKeyboardActivation: события — литералы-срезы
 * {key, repeat, target, currentTarget, preventDefault}, ни React, ни DOM
 * (прецедент — useLongPress.test.ts).
 */

type TestKeyDownEvent = {
  readonly key: string;
  readonly repeat: boolean;
  readonly target: unknown;
  readonly currentTarget: unknown;
  readonly preventDefault: () => void;
};

const ACTIVATION_KEYS = ['Enter', ' '] as const;

/** Сама строка-див (currentTarget обработчика) и сфокусированная вложенная
 * кнопка (звезда «Убрать из избранного»): keydown таргетится на
 * сфокусированный элемент, литералы — наглядные маркеры, не DOM. */
const ROW = { element: 'row' };
const STAR = { element: 'star' };

const keyDown = (overrides: Partial<TestKeyDownEvent>): TestKeyDownEvent => ({
  key: 'Enter',
  repeat: false,
  target: ROW,
  currentTarget: ROW,
  preventDefault: () => undefined,
  ...overrides,
});

describe('createKeyboardActivation guard авто-повтора (#833)', () => {
  it.each(ACTIVATION_KEYS)(
    'удержание %j активирует onSelect ровно один раз — авто-повторы глушатся',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
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
      const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
        onSelect,
      });
      const prevented = vi.fn();

      onKeyDown?.(keyDown({ key, repeat: true, preventDefault: prevented }));

      expect(onSelect).not.toHaveBeenCalled();
      expect(prevented).toHaveBeenCalledOnce();
    },
  );

  it.each(ACTIVATION_KEYS)(
    'первое нажатие %j на самой строке — прежний контракт: onSelect + preventDefault',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
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
    const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
      onSelect,
    });
    const prevented = vi.fn();

    onKeyDown?.(
      keyDown({ key: 'ArrowDown', repeat: true, preventDefault: prevented }),
    );

    expect(onSelect).not.toHaveBeenCalled();
    expect(prevented).not.toHaveBeenCalled();
  });
});

describe('createKeyboardActivation keydown из вложенного интерактивного элемента (#831)', () => {
  it.each(ACTIVATION_KEYS)(
    'keydown %j сфокусированной вложенной кнопки не активирует строку и не preventDefault — активационный click кнопки не давится',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
        onSelect,
      });
      const prevented = vi.fn();

      onKeyDown?.(
        keyDown({ key, target: STAR, preventDefault: prevented }),
      );

      expect(onSelect).not.toHaveBeenCalled();
      expect(prevented).not.toHaveBeenCalled();
    },
  );
  // Контр-случай (target === currentTarget активирует) — «первое нажатие
  // %j на самой строке» в describe guard авто-повтора: тот же литерал.
});

describe('createKeyboardActivation неинтерактивная строка', () => {
  it('без onSelect onKeyDown не навешивается', () => {
    const activation = createKeyboardActivation<TestKeyDownEvent>({});

    expect(activation.onKeyDown).toBeUndefined();
    expect(activation.role).toBeUndefined();
    expect(activation.tabIndex).toBeUndefined();
  });

  it('disabled=true: onKeyDown не навешивается, aria-disabled выставлен', () => {
    const onSelect = vi.fn();
    const activation = createKeyboardActivation<TestKeyDownEvent>({
      onSelect,
      disabled: true,
    });

    expect(activation.onKeyDown).toBeUndefined();
    expect(activation['aria-disabled']).toBe(true);
    expect(activation.role).toBe('button');
    expect(activation.tabIndex).toBeUndefined();
  });
});
