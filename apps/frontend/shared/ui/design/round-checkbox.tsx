import type { ComponentProps, JSX } from 'react';
import { CheckboxIndicator, Checkbox as CheckboxRoot } from '@radix-ui/react-checkbox';
import { Check } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Круглый чекбокс дизайн-слоя — «Selection Button» (Figma 1031:21053,
 * Variant=Radio): кружок 24×24 с серой обводкой 1.5 → синяя заливка с белой
 * галочкой 16×16; нажатие — тёмно-синий #176BEB. Строка задач получает его
 * ведущим элементом: тап по кружку выполняет/снимает задачу, тап по строке
 * — редактирование (карта #497), поэтому кружок живёт в своём stopPropagation
 * -контейнере на стороне потребителя. Радикс даёт Space-переключение и
 * role=checkbox. */
export type RoundCheckboxProps = ComponentProps<typeof CheckboxRoot>;

export function RoundCheckbox({ className, ...props }: RoundCheckboxProps): JSX.Element {
  return (
    <CheckboxRoot
      className={cn(
        'flex h-6 w-6 shrink-0 cursor-pointer items-center justify-center rounded-pill border-[1.5px] border-content-tertiary bg-transparent font-sans outline-none transition-colors',
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
