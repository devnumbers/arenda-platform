import type { ComponentProps, JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

/** Чип-кнопка дизайн-слоя (Figma 709:15742): пилюля radius 100 на сером
 * фоне с иконками 24×24 и текстом 14/16; выбранное состояние (Enable) —
 * синяя заливка с белым текстом, aria-pressed у кнопки. */
export type ChipButtonProps = ComponentProps<'button'> & {
  readonly leadingIcon?: ReactNode;
  readonly trailingIcon?: ReactNode;
  readonly selected?: boolean;
};

export function ChipButton({
  className,
  leadingIcon,
  trailingIcon,
  selected = false,
  type = 'button',
  children,
  ...props
}: ChipButtonProps): JSX.Element {
  return (
    <button
      type={type}
      aria-pressed={selected}
      className={cn(
        'inline-flex h-11 shrink-0 cursor-pointer items-center justify-center gap-1 rounded-pill bg-surface-muted px-5 font-sans text-sm font-medium text-content outline-none transition-colors',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'disabled:pointer-events-none disabled:opacity-50',
        selected && 'bg-primary text-white',
        className,
      )}
      {...props}
    >
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
    </button>
  );
}
