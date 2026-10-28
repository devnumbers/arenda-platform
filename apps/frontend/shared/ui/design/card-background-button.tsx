import type { JSX } from 'react';

/**
 * Фоновая кнопка кликабельной серой карточки: клик по свободному фону
 * карточки ведёт туда же, куда её заголовок (решение владельца 28.10 —
 * секции «Платежей объекта»/хаба, затем «аренды»). Лежит ПОД контентом
 * (позиционный sibling раньше обёртки контента), контент-обёртка —
 * pointer-events-none, интерактивные дети возвращают pointer-events-auto
 * себе: клики по строкам до фона не доходят. Вне tab-порядка и скрыта от
 * AT — клавиатурный путь карточки — нативная кнопка заголовка.
 */
export function CardBackgroundButton({
  onOpen,
}: {
  readonly onOpen: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onOpen}
      aria-hidden
      tabIndex={-1}
      className="absolute inset-0 cursor-pointer rounded-card outline-none"
    />
  );
}
