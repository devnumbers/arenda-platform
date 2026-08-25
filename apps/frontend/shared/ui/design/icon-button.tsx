import type { ComponentProps, JSX, ReactNode } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/shared/lib/cn';

/**
 * Кнопка-иконка дизайн-слоя (Figma 934:19148). Круглая зона нажатия 44×44
 * под иконку 24×24: Primary — прозрачная с серым hover, Secondary —
 * постоянный серый круг (hover/active не меняют — так в Figma), Danger —
 * прозрачная с красной иконкой. Focus-visible — синее кольцо 2px без смещения.
 */
const iconButtonVariants = cva(
  'inline-flex shrink-0 items-center justify-center rounded-pill font-sans outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        primary: 'text-content hover:bg-surface-muted active:bg-surface-muted-hover',
        secondary: 'bg-surface-muted text-content',
        danger: 'text-danger',
      },
    },
    defaultVariants: {
      variant: 'primary',
    },
  },
);

export type IconButtonProps = ComponentProps<'button'> &
  VariantProps<typeof iconButtonVariants> & {
    readonly icon: ReactNode;
    /** Имя для screen reader — кнопка-иконка без текста обязана его иметь. */
    readonly label: string;
  };

export function IconButton({
  className,
  variant,
  icon,
  label,
  type = 'button',
  ...props
}: IconButtonProps): JSX.Element {
  return (
    <button
      type={type}
      aria-label={label}
      className={cn(iconButtonVariants({ variant }), 'h-11 w-11', className)}
      {...props}
    >
      <span className="flex h-6 w-6 items-center justify-center" aria-hidden>
        {icon}
      </span>
    </button>
  );
}
