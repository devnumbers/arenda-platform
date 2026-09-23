import type { ComponentProps, JSX } from 'react';
import { CheckboxIndicator, Checkbox as CheckboxRoot } from '@radix-ui/react-checkbox';
import { Check, Minus } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/**
 * Чекбокс дизайн-слоя (Figma 1031:21053): квадрат 20×20 radius 8,
 * серая обводка → синий заливкой с белой галочкой 16×16; нажатие —
 * тёмно-синий #176BEB. Радикс даёт Space-переключение и role=checkbox.
 *
 * Частичный выбор (indeterminate, #711 — мастер-чекбокс группы фильтров
 * истории, макет 2067-162950): синий квадрат с белым минусом 16×16 —
 * переключение `checked="indeterminate"`, стили по data-state.
 *
 * Disabled (#712 — прибитый участник в шите «Действий участника», макет
 * 2184-92510, решение владельца 23.09): выбранный — серый квадрат #D3D7D9
 * с белой галочкой/минусом, без прозрачности (невыбранный — прежняя
 * полупрозрачность); клики не проходят.
 */
export type CheckboxProps = ComponentProps<typeof CheckboxRoot>;

export function Checkbox({ className, checked, ...props }: CheckboxProps): JSX.Element {
  return (
    <CheckboxRoot
      checked={checked}
      className={cn(
        'flex h-5 w-5 shrink-0 cursor-pointer items-center justify-center rounded-lg border-[1.5px] border-content-tertiary bg-transparent font-sans outline-none transition-colors',
        'hover:border-primary',
        'active:border-primary-active active:bg-primary-active',
        'data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-white',
        'data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary data-[state=indeterminate]:text-white',
        'data-[state=checked]:hover:bg-primary-hover data-[state=checked]:active:bg-primary-active data-[state=checked]:active:border-primary-active',
        'data-[state=indeterminate]:hover:bg-primary-hover data-[state=indeterminate]:active:bg-primary-active data-[state=indeterminate]:active:border-primary-active',
        'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
        'disabled:pointer-events-none disabled:data-[state=unchecked]:opacity-50',
        'disabled:data-[state=checked]:border-[#D3D7D9] disabled:data-[state=checked]:bg-[#D3D7D9]',
        'disabled:data-[state=indeterminate]:border-[#D3D7D9] disabled:data-[state=indeterminate]:bg-[#D3D7D9]',
        className,
      )}
      {...props}
    >
      <CheckboxIndicator>
        {checked === 'indeterminate' ? (
          <Minus className="h-4 w-4" aria-hidden />
        ) : (
          <Check className="h-4 w-4" aria-hidden />
        )}
      </CheckboxIndicator>
    </CheckboxRoot>
  );
}
