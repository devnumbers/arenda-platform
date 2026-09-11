import type { KeyboardEvent, ReactNode } from 'react';
import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { runListboxAction } from '@/shared/ui/select/listbox-keyboard';

/** Строка списка дизайн-слоя (Figma 699:7254): слоты — ведущий элемент
 * (иконка категории 44×44 кладётся слотом), заголовок + подзаголовок с
 * мини-иконкой, значение + описание справа, ведомый элемент (иконки
 * действий). Горизонтальный паддинг 24 встроен, как в Figma-компоненте;
 * hover приглушает заголовок и значение (#6F787C).
 *
 * Строка — div с role=button, а не <button>: trailing-слот несёт
 * собственные кнопки-иконки, а вложенные кнопки в HTML невалидны (ловится
 * hydration-ошибкой). Клавиатура: Enter/Space вызывают onSelect.
 *
 * Вариант option — строка popup-списка (listbox, подсказки адреса):
 * role=option внутри контейнера role=listbox, roving focus (Tab список
 * обходит, входят стрелками с поля), aria-selected=false (кандидат, а не
 * выбор), клавиатура — общий listbox-модуль: стрелки с wrap-around,
 * Home/End, Enter/Space; Escape не гасится — всплывает контейнеру списка,
 * который закрывает список и возвращает фокус полю. */
export type ListRowProps = {
  readonly leading?: ReactNode;
  readonly title: ReactNode;
  readonly subtitle?: ReactNode;
  readonly subtitleIcon?: ReactNode;
  readonly value?: ReactNode;
  readonly description?: ReactNode;
  readonly trailing?: ReactNode;
  /** Основное действие строки; без него строка неинтерактивна. */
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
  readonly className?: string;
  /** Дополнение к классу подписи (другой цвет из того же компонента Row
   * Button — подсказки адреса несут #6F787C, Figma 1519:94336). */
  readonly subtitleClassName?: string;
  /** Дополнение к классу заголовка (другой цвет из того же компонента Row
   * Button — выбранная зона в пикере часового пояса несёт #2B7FFF,
   * Figma 1869:71099, #594). */
  readonly titleClassName?: string;
  /** Роль строки: option — внутри role=listbox (см. выше), button —
   * обычная строка-кнопка (по умолчанию). */
  readonly variant?: 'button' | 'option';
};

export function ListRow({
  leading,
  title,
  subtitle,
  subtitleClassName,
  subtitleIcon,
  value,
  description,
  trailing,
  onSelect,
  disabled = false,
  className,
  titleClassName,
  variant = 'button',
}: ListRowProps): JSX.Element {
  const interactive = onSelect !== undefined && !disabled;
  const isOption = variant === 'option';

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (!interactive) {
      return;
    }
    // Опция списка ведёт клавиатуру через общий listbox-модуль (стрелки,
    // Home/End, Enter/Space; Escape всплывает контейнеру списка).
    if (isOption) {
      runListboxAction(event, { onSelect });
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onSelect();
    }
  };

  return (
    <div
      role={isOption ? 'option' : onSelect !== undefined ? 'button' : undefined}
      aria-selected={isOption ? false : undefined}
      tabIndex={interactive ? (isOption ? -1 : 0) : undefined}
      aria-disabled={disabled || undefined}
      onClick={interactive ? onSelect : undefined}
      onKeyDown={handleKeyDown}
      className={cn(
        'group/row flex w-full cursor-pointer items-center gap-3 px-6 py-2 text-left font-sans outline-none',
        'transition-colors focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        onSelect === undefined && 'cursor-default',
        disabled && 'pointer-events-none opacity-50',
        className,
      )}
    >
      {leading !== undefined && <span className="flex shrink-0 items-center">{leading}</span>}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span
          className={cn(
            'truncate text-base font-medium text-content group-hover/row:text-content-secondary',
            titleClassName,
          )}
        >
          {title}
        </span>
        {subtitle !== undefined && (
          <span
            className={cn(
              'flex items-center gap-1.5 text-sm text-content-tertiary',
              subtitleClassName,
            )}
          >
            {subtitleIcon !== undefined && (
              <span className="flex h-4 w-4 shrink-0 items-center justify-center" aria-hidden>
                {subtitleIcon}
              </span>
            )}
            <span className="truncate">{subtitle}</span>
          </span>
        )}
      </span>
      {(value !== undefined || description !== undefined) && (
        <span className="flex shrink-0 flex-col items-end gap-1">
          {value !== undefined && (
            <span className="text-base font-medium text-content group-hover/row:text-content-secondary">
              {value}
            </span>
          )}
          {description !== undefined && <span className="text-sm text-content-tertiary">{description}</span>}
        </span>
      )}
      {trailing !== undefined && <span className="flex shrink-0 items-center">{trailing}</span>}
    </div>
  );
}
