import type { ComponentProps, JSX } from 'react';
import { CheckboxIndicator, Checkbox as CheckboxRoot } from '@radix-ui/react-checkbox';
import { Check } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Чекбокс дизайн-слоя (Figma 1031:21053): квадрат 20×20 radius 8,
 * серая обводка → синий заливкой с белой галочкой 16×16; нажатие —
 * тёмно-синий #176BEB. Радикс даёт Space-переключение и role=checkbox. */
export type CheckboxProps = ComponentProps<typeof CheckboxRoot>;

export function Checkbox({ className, ...props }: CheckboxProps): JSX.Element {
  return (
    <CheckboxRoot
      className={cn(
        'flex h-5 w-5 shrink-0 cursor-pointer items-center justify-center rounded-lg border-[1.5px] border-content-tertiary bg-transparent font-sans outline-none transition-colors',
        'hover:border-primary',
        'active:border-primary-active active:bg-primary-active',
        'data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-white',
        'data-[state=checked]:hover:bg-primary-hover data-[state=checked]:active:bg-primary-active data-[state=checked]:active:border-primary-active',
        'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <CheckboxIndicator>
        <Check className="h-4 w-4" aria-hidden />
      </CheckboxIndicator>
    </CheckboxRoot>
  );
}
