import type { ComponentProps, JSX, ReactNode } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { Loading } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/**
 * Кнопка дизайн-слоя (ADR 0050, Figma 939:46118 — «Рентли. Новые экраны
 * сервиса»). Варианты Primary/Secondary/Danger/Clear, размер default —
 * прямоугольник radius 16, small — пилюля radius 100. Focus-visible —
 * каноничный ring дизайн-системы: 4px синий поверх 2px белого смещения.
 */

/** Заливка Primary — базовый вариант и выбранное состояние пилюли (Figma
 * State=Enable), один источник на оба применения. */
const primaryFill = 'bg-primary text-white hover:bg-primary-hover active:bg-primary-active';

const buttonVariants = cva(
  'inline-flex cursor-pointer items-center justify-center whitespace-nowrap font-sans font-medium transition-colors outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        primary: primaryFill,
        secondary:
          'bg-surface-muted text-content hover:bg-surface-muted-hover active:bg-surface-muted-active',
        danger: 'bg-surface-danger text-danger',
        clear: 'rounded-pill bg-surface text-content-secondary',
      },
      size: {
        default: 'h-12 gap-2 rounded-button px-6 text-base',
        small: 'h-11 gap-1.5 rounded-pill px-4 text-sm',
      },
      selected: {
        false: '',
        true: primaryFill,
      },
    },
    compoundVariants: [
      // Clear в default-размере остаётся пилюлей с зазором 6px (Figma
      // 1134:55051), а не прямоугольником 16px с зазором 8px.
      { variant: 'clear', size: 'default', class: 'gap-1.5 rounded-pill' },
    ],
    defaultVariants: {
      variant: 'primary',
      size: 'default',
      selected: false,
    },
  },
);

export type ButtonProps = ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    readonly asChild?: boolean;
    readonly leadingIcon?: ReactNode;
    readonly trailingIcon?: ReactNode;
    readonly loading?: boolean;
  };

export function Button({
  className,
  variant,
  size,
  selected,
  asChild = false,
  leadingIcon,
  trailingIcon,
  loading = false,
  disabled,
  children,
  ...props
}: ButtonProps): JSX.Element {
  const Comp = asChild ? Slot : 'button';

  return (
    <Comp
      data-loading={loading ? 'true' : undefined}
      aria-busy={loading || undefined}
      className={cn(buttonVariants({ variant, size, selected }), className)}
      disabled={asChild ? undefined : (disabled ?? loading)}
      {...props}
    >
      {loading ? (
        <Loading className="h-6 w-6 animate-spin" aria-hidden />
      ) : (
        <>
          {leadingIcon !== undefined && (
            <span className="flex h-6 w-6 shrink-0 items-center justify-center" aria-hidden>
              {leadingIcon}
            </span>
          )}
          {children}
          {trailingIcon !== undefined && (
            <span className="flex h-6 w-6 shrink-0 items-center justify-center" aria-hidden>
              {trailingIcon}
            </span>
          )}
        </>
      )}
    </Comp>
  );
}
