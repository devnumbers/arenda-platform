import { describe, expect, it, vi } from 'vitest';
import { createKeyboardActivation } from './useKeyboardActivation';

/**
 * Контракт клавиатурной активации строк-кнопок (тикет #833). Хук — тонкая
 * обёртка без React-состояния, поведенческие гарантии пинятся на чистой
 * фабрике createKeyboardActivation: события — литералы-срезы
 * {key, repeat, preventDefault}, ни React, ни DOM (прецедент —
 * useLongPress.test.ts).
 */

type TestKeyDownEvent = {
  readonly key: string;
  readonly repeat: boolean;
  readonly preventDefault: () => void;
};

const ACTIVATION_KEYS = ['Enter', ' '] as const;

describe('createKeyboardActivation guard авто-повтора (#833)', () => {
  it.each(ACTIVATION_KEYS)(
    'удержание %j активирует onSelect ровно один раз — авто-повторы глушатся',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
        onSelect,
      });

      onKeyDown?.({ key, repeat: false, preventDefault: () => undefined });
      for (let i = 0; i < 5; i += 1) {
        onKeyDown?.({ key, repeat: true, preventDefault: () => undefined });
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

      onKeyDown?.({ key, repeat: true, preventDefault: prevented });

      expect(onSelect).not.toHaveBeenCalled();
      expect(prevented).toHaveBeenCalledOnce();
    },
  );

  it.each(ACTIVATION_KEYS)(
    'первое нажатие %j — прежний контракт: onSelect + preventDefault',
    (key) => {
      const onSelect = vi.fn();
      const { onKeyDown } = createKeyboardActivation<TestKeyDownEvent>({
        onSelect,
      });
      const prevented = vi.fn();

      onKeyDown?.({ key, repeat: false, preventDefault: prevented });

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

    onKeyDown?.({ key: 'ArrowDown', repeat: true, preventDefault: prevented });

    expect(onSelect).not.toHaveBeenCalled();
    expect(prevented).not.toHaveBeenCalled();
  });
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
