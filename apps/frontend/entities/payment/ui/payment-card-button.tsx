import type { ReactNode } from 'react';
import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

/**
 * Карточка платежа (компонент Figma «Payment Card Button», 705:10625 —
 * резолюция #449): серая плитка 168.5px с иконкой категории (слот `leading`,
 * в Figma — кант по серой поверхности), суммой и датой сверху, названием
 * платежа и объектом снизу. Горизонтальный скролл секции «Просроченные».
 * Стили просрочки (`danger`) — сумма и срок красным (#452); danger-бейдж
 * на иконке категорий остаётся её пропом. Без `amountKopecks` верх остаётся
 * только иконкой, а `accentTitle` красит название синим (варианты «Blue
 * Title» 879:17555/879:17565 — замыкающие карточки секций главного экрана
 * «Платежи», #578).
 */
export type PaymentCardButtonProps = {
  /** Заголовок карточки, например название платежа. */
  readonly title: ReactNode;
  /** Подпись под заголовком, например название объекта. */
  readonly subtitle?: ReactNode;
  /** Сумма; не задана — строки суммы нет (карточки «Все …»/«Показать все»). */
  readonly amountKopecks?: number;
  /** Срок под суммой: дата или «N дней» просрочки. */
  readonly description?: ReactNode;
  readonly leading?: ReactNode;
  /** Просроченная операция: сумма и срок красным (#452). */
  readonly danger?: boolean;
  /** Название синим (879:17555/879:17565) — замыкающая карточка секции. */
  readonly accentTitle?: boolean;
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
  readonly className?: string;
};

export function PaymentCardButton({
  title,
  subtitle,
  amountKopecks,
  description,
  leading,
  danger = false,
  accentTitle = false,
  onSelect,
  disabled = false,
  className,
}: PaymentCardButtonProps): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect, disabled });

  return (
    <div
      {...activatorProps}
      className={cn(
        'flex w-[168.5px] shrink-0 cursor-pointer flex-col rounded-card bg-surface-muted p-4 outline-none',
        'transition-all focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80 active:opacity-80',
        onSelect === undefined && 'cursor-default',
        disabled && 'pointer-events-none opacity-50',
        className,
      )}
    >
      <span className="flex w-full pb-3">
        <span className="flex min-w-0 flex-1 items-center gap-2">
          {leading !== undefined && <span className="shrink-0">{leading}</span>}
          {amountKopecks !== undefined && (
            <span className="flex min-w-0 flex-1 flex-col justify-center gap-1 text-xs font-medium">
              <span className={cn('truncate', danger ? 'text-danger' : 'text-content')}>
                {formatMoneyKopecks(amountKopecks)}
              </span>
              {description !== undefined && (
                <span
                  className={cn(
                    'truncate',
                    danger ? 'text-danger' : 'text-content-tertiary',
                  )}
                >
                  {description}
                </span>
              )}
            </span>
          )}
        </span>
      </span>
      <span className="flex w-full flex-col gap-1 text-xs font-medium">
        <span className={cn('line-clamp-2', accentTitle ? 'text-primary' : 'text-content')}>
          {title}
        </span>
        {subtitle !== undefined && (
          <span className="truncate text-content-tertiary">{subtitle}</span>
        )}
      </span>
    </div>
  );
}
