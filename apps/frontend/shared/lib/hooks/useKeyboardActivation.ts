import type { KeyboardEvent } from 'react';

/**
 * Клавиатурная активация строк-кнопок дизайн-слоя (ListRow, PaymentRowButton,
 * PaymentCardButton): строка — div с role=button, потому что trailing-слот
 * несёт собственные кнопки, а вложенные кнопки в HTML невалидны. Enter/Space
 * вызывают onSelect; без onSelect строка неинтерактивна (role/tabIndex
 * не навешиваются).
 *
 * Логика активации живёт в чистой фабрике createKeyboardActivation:
 * события — обычные аргументы, никаких React-рефов и DOM (юнит-слой
 * тестируется в node); useKeyboardActivation — тонкая обёртка
 * (прецедент — createLongPress/useLongPress).
 */
/** Срез keydown-события, достаточный активации: фабрика не знает ни
 * React-событий, ни DOM — хук подставляет React-событие, тесты —
 * литералы. */
export type KeyboardActivationEvent = {
  readonly key: string;
  readonly repeat: boolean;
  /** Элемент, на котором случился keydown: у keydown это сфокусированный
   * элемент. Равен currentTarget, когда фокус на самой строке; иначе
   * keydown всплыл из вложенного сфокусированного элемента. */
  readonly target: unknown;
  /** Элемент-строка, на который навешан onKeyDown. */
  readonly currentTarget: unknown;
  readonly preventDefault: () => void;
};

export type KeyboardActivationOptions = {
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
};

export type CreatedKeyboardActivation<E extends KeyboardActivationEvent> = {
  /** ARIA-роль строки-кнопки: 'button' при наличии onSelect; строка без
   * onSelect неинтерактивна и роль не навешивается. */
  readonly role: 'button' | undefined;
  /** Включение в tab-порядок: 0 только у интерактивной строки (onSelect
   * есть и не disabled), иначе не навешивается. */
  readonly tabIndex: 0 | undefined;
  /** Признак недоступной строки: true только при disabled, у включённой
   * атрибут не навешивается. */
  readonly 'aria-disabled': boolean | undefined;
  /** Активация кликом мыши/тача — прокидывается сам onSelect: клик и
   * клавиатура (Enter/Space) делают одно и то же действие; иначе не
   * навешивается. */
  readonly onClick: (() => void) | undefined;
  /** Клавиатурная активация: Enter/Space вызывают onSelect; keydown,
   * всплывший из вложенного сфокусированного элемента, строку не
   * активирует (#831), авто-повтор удержания глушится (#833). У
   * неинтерактивной строки не навешивается. */
  readonly onKeyDown: ((event: E) => void) | undefined;
};

/** React-обёртка над фабричным типом: событие —
 * KeyboardEvent<HTMLDivElement>; докстринги полей — у фабричного типа
 * выше, здесь не дублируются. */
export type KeyboardActivatorProps =
  CreatedKeyboardActivation<KeyboardEvent<HTMLDivElement>>;

export function createKeyboardActivation<E extends KeyboardActivationEvent>({
  onSelect,
  disabled = false,
}: KeyboardActivationOptions): CreatedKeyboardActivation<E> {
  const interactive = onSelect !== undefined && !disabled;

  const handleKeyDown = (event: E): void => {
    if (!interactive) {
      return;
    }
    if (event.target !== event.currentTarget) {
      // Keydown всплыл из вложенного сфокусированного элемента (например,
      // звезда «Убрать из избранного» в /payments/favorites): активация —
      // дело самого элемента, а preventDefault строки давил бы его
      // нативный активационный click (#831).
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      if (event.repeat) {
        // Авто-повтор удержания — не повторное нажатие: onSelect
        // срабатывает один раз, повторы глушатся (#833); preventDefault
        // выше остаётся, чтобы удержание не скроллило и не активировало
        // нативно.
        return;
      }
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

export function useKeyboardActivation(
  options: KeyboardActivationOptions,
): KeyboardActivatorProps {
  return createKeyboardActivation<KeyboardEvent<HTMLDivElement>>(options);
}
