import type { ComponentProps, JSX, ReactNode } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { Loading } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/**
 * Кнопка дизайн-слоя (ADR 0050, Figma 939:46118 — «Рентли. Новые экраны
 * сервиса»). Два размера: default — 56px, radius 16, паддинг 24, зазор 8,
 * кегль R/500 16/18; small — 44px, пилюля radius 100, паддинг 20, зазор 6,
 * кегль M/500 14/16. Варианты Primary (#2B7FFF→hover #2175F5→active
 * #176BEB), Secondary (серый фон с шагами hover/active), Danger (розовый
 * фон, красный текст — hover/active фон не меняют), Clear (белая пилюля,
 * серый текст) и White (белая пилюля, тёмный текст); Clear/White — пилюли
 * с кеглем M в ОБОИХ размерах (Figma 1134:55051, 1185:41649). Focus-visible
 * — ring 4px синий поверх 2px белого смещения (State/Focus), только с
 * клавиатуры.
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
        clear: 'bg-surface text-content-secondary',
        white: 'bg-surface text-content',
      },
      size: {
        default: 'h-14 gap-2 rounded-button px-6 text-base',
        small: 'h-11 gap-1.5 rounded-pill px-5 text-sm',
      },
      selected: {
        false: '',
        true: primaryFill,
      },
    },
    compoundVariants: [
      // Clear и White — пилюли с кеглем M/500 и зазором 6 в обоих размерах
      // (Figma 1134:55051, 1185:41649), не прямоугольники 16px/8px.
      {
        variant: ['clear', 'white'],
        class: 'gap-1.5 rounded-pill text-sm',
      },
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
