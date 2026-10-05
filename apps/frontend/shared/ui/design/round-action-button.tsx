import type { ComponentProps, JSX, ReactNode } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/shared/lib/cn';

/**
 * Круглая кнопка-действие (Figma «Primary Round Button», 671:6212 —
 * резолюция #452, страница платежа): круг 56×56 с иконкой 24×24 и подпись
 * Onest M/500 13/15 под ним; кликабельна вся колонка. Secondary — серый
 * круг #F3F4F6 («На паузу», «Возобновить», «Изменить»), Primary — синий
 * #2B7FFF с белой иконкой («Оплатить»). Focus-visible — как у Button,
 * только с клавиатуры. loading глушит кнопку при любом явном disabled
 * (disabled || loading, #1119) — канон ADR 0050 машинно.
 */

const roundCircleVariants = cva(
  'flex h-14 w-14 items-center justify-center rounded-pill transition-colors',
  {
    variants: {
      variant: {
        primary:
          'bg-primary text-white group-hover/round:bg-primary-hover group-active/round:bg-primary-active',
        secondary:
          'bg-surface-muted text-content group-hover/round:bg-surface-muted-hover group-active/round:bg-surface-muted-active',
      },
    },
    defaultVariants: {
      variant: 'secondary',
    },
  },
);

export type RoundActionButtonProps = ComponentProps<'button'> &
  VariantProps<typeof roundCircleVariants> & {
    readonly icon: ReactNode;
    /** Подпись под кругом: «На паузу», «Оплатить» и т.д.; часть имени
     * кнопки. ReactNode — многострочные подписи макета (решение #802:
     * «Завершить/аренду» переносом). */
    readonly caption: ReactNode;
    readonly loading?: boolean;
  };

export function RoundActionButton({
  className,
  variant,
  icon,
  caption,
  loading = false,
  disabled,
  type = 'button',
  ...props
}: RoundActionButtonProps): JSX.Element {
  return (
    <button
      type={type}
      aria-busy={loading || undefined}
      disabled={disabled === true || loading}
      className={cn(
        'group/round inline-flex cursor-pointer flex-col items-center gap-2 outline-none',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <span className={roundCircleVariants({ variant })} aria-hidden>
        <span className="flex h-6 w-6 items-center justify-center">{icon}</span>
      </span>
      <span className="text-center text-[13px] leading-[15px] font-medium text-content">
        {caption}
      </span>
    </button>
  );
}
