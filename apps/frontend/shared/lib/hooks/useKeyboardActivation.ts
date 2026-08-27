import type { KeyboardEvent } from 'react';

/**
 * Клавиатурная активация строк-кнопок дизайн-слоя (ListRow, PaymentRowButton,
 * PaymentCardButton): строка — div с role=button, потому что trailing-слот
 * несёт собственные кнопки, а вложенные кнопки в HTML невалидны. Enter/Space
 * вызывают onSelect; без onSelect строка неинтерактивна (role/tabIndex
 * не навешиваются).
 */
export type KeyboardActivationOptions = {
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
};

export type KeyboardActivatorProps = {
  readonly role: 'button' | undefined;
  readonly tabIndex: 0 | undefined;
  readonly 'aria-disabled': boolean | undefined;
  readonly onClick: (() => void) | undefined;
  readonly onKeyDown: ((event: KeyboardEvent<HTMLDivElement>) => void) | undefined;
};

export function useKeyboardActivation({
  onSelect,
  disabled = false,
}: KeyboardActivationOptions): KeyboardActivatorProps {
  const interactive = onSelect !== undefined && !disabled;

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (!interactive) {
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onSelect();
    }
  };

  return {
    role: onSelect !== undefined ? 'button' : undefined,
    tabIndex: interactive ? 0 : undefined,
    'aria-disabled': disabled || undefined,
    onClick: interactive ? onSelect : undefined,
    onKeyDown: interactive ? handleKeyDown : undefined,
  };
}
