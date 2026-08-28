import type { ReactNode } from 'react';
import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

/**
 * Строка операции платежа (компонент Figma «Payment Row Button», 936:39348 —
 * резолюция #449): необязательные слоты ведущего элемента и действий,
 * иконка категории 44×44 (слот `categoryIcon`), заголовок + подзаголовок
 * со своей мини-иконкой, справа сумма + описание (дата или срок просрочки).
 * Поверхности White/Gray; hover и press приглушают строку до opacity 80%.
 * Стили просрочки (`danger`) — сумма и описание красным (#452).
 */
export type PaymentRowButtonProps = {
  readonly categoryIcon?: ReactNode;
  readonly title: ReactNode;
  /** Подзаголовок, например название объекта рядом с избранной звездой. */
  readonly subtitle?: ReactNode;
  readonly subtitleIcon?: ReactNode;
  /** Ведущий слот перед контентным фреймом (кнопка-иконка и т.п.). */
  readonly leading?: ReactNode;
  readonly trailing?: ReactNode;
  /** Сумма в копейках, форматируется каноническим money-форматтером. */
  readonly amountKopecks?: number;
  /** Правое нижнее поле — дата планового вхождения или срок просрочки. */
  readonly description?: ReactNode;
  readonly variant?: 'white' | 'gray';
  /** Просроченная операция: сумма и описание красным (#452). */
  readonly danger?: boolean;
  /** Знаковая сумма операции (1332:61665, State=Plus): положительная — с
   * плюсом зелёным (доход). У плашек правил не включается — их сумма всегда
   * без знака. */
  readonly signedAmount?: boolean;
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
  readonly className?: string;
};

export function PaymentRowButton({
  categoryIcon,
  title,
  subtitle,
  subtitleIcon,
  leading,
  trailing,
  amountKopecks,
  description,
  variant = 'white',
  danger = false,
  signedAmount = false,
  onSelect,
  disabled = false,
  className,
}: PaymentRowButtonProps): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect, disabled });

  return (
    <div
      {...activatorProps}
      className={cn(
        'group/row flex w-full cursor-pointer items-center py-2 outline-none',
        'transition-all focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 active:opacity-80 px-3',
        variant === 'gray' ? 'bg-surface-muted' : 'bg-surface',
        onSelect === undefined && 'cursor-default',
        disabled && 'pointer-events-none opacity-50',
        className,
      )}
    >
      {leading !== undefined && <span className="flex shrink-0">{leading}</span>}
      <span className="flex min-w-0 flex-1 items-center px-3">
        <span className="flex min-w-0 flex-1 items-center gap-3">
          {categoryIcon !== undefined && <span className="shrink-0">{categoryIcon}</span>}
          <span className="flex min-w-0 flex-1 items-center gap-2">
            <span className="flex min-w-0 flex-1 flex-col justify-center gap-1">
              <span className="truncate text-base font-medium text-content">{title}</span>
              {subtitle !== undefined && (
                <span className="flex items-center gap-1 text-sm text-content-secondary">
                  {subtitleIcon !== undefined && (
                    <span className="flex h-4 w-4 shrink-0 items-center justify-center" aria-hidden>
                      {subtitleIcon}
                    </span>
                  )}
                  <span className="truncate">{subtitle}</span>
                </span>
              )}
            </span>
            {(amountKopecks !== undefined || description !== undefined) && (
              <span className="flex shrink-0 flex-col items-end gap-1">
                {amountKopecks !== undefined && (
                  <span
                    className={cn(
                      'text-base font-medium',
                      danger
                        ? 'text-danger'
                        : signedAmount && amountKopecks > 0
                          ? 'text-success'
                          : 'text-content group-hover/row:text-content-secondary',
                    )}
                  >
                    {signedAmount && amountKopecks > 0 && '+'}
                    {formatMoneyKopecks(amountKopecks)}
                  </span>
                )}
                {description !== undefined && (
                  <span
                    className={cn(
                      'text-sm',
                      danger ? 'font-medium text-danger' : 'text-content-tertiary',
                    )}
                  >
                    {description}
                  </span>
                )}
              </span>
            )}
          </span>
        </span>
      </span>
      {trailing !== undefined && <span className="flex shrink-0">{trailing}</span>}
    </div>
  );
}
