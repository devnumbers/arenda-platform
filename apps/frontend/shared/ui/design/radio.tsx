import type { ComponentProps, JSX } from 'react';
import {
  RadioGroup as RadioGroupRoot,
  RadioGroupIndicator,
  RadioGroupItem as RadioGroupItemRoot,
} from '@radix-ui/react-radio-group';
import { Check } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Радио дизайн-слоя (Figma 1031:21053): круг 20×20, по выбору — синий
 * с белой галочкой (глиф чека — так нарисован Figma-компонент, не точка).
 * Стрелки клавиатуры по группе даёт Radix RadioGroup. */

export type RadioGroupProps = ComponentProps<typeof RadioGroupRoot>;

export function RadioGroup({ className, ...props }: RadioGroupProps): JSX.Element {
  return <RadioGroupRoot className={cn('flex flex-col gap-2 font-sans', className)} {...props} />;
}

export type RadioGroupItemProps = ComponentProps<typeof RadioGroupItemRoot>;

export function RadioGroupItem({ className, ...props }: RadioGroupItemProps): JSX.Element {
  return (
    <RadioGroupItemRoot
      className={cn(
        'flex h-5 w-5 shrink-0 items-center justify-center rounded-pill border-[1.5px] border-content-tertiary bg-transparent font-sans outline-none transition-colors',
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
      <RadioGroupIndicator>
        <Check className="h-4 w-4" aria-hidden />
      </RadioGroupIndicator>
    </RadioGroupItemRoot>
  );
}
