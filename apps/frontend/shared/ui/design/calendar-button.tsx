import type { ComponentProps, JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Кнопка дня календаря дизайн-слоя (Figma 766:9738, выверено рендером
 * инстансов 1:1): ячейка 48×48, radius 12, кегль R/500 16/18. Состояния
 * по сету: Default #FFFFFF → hover #F3F4F6 → active #E9EAEC; selected —
 * заливка #2B7FFF с белым текстом (hover/active — шаги primary); today —
 * маркер #EFF6FF без hover-переходов (в сете их нет); disabled — 50%.
 * Focus-visible — обводка 2px #2B7FFF, только с клавиатуры (фокус-узел
 * сета отрисован без заливки; клавиатурная метка — конвенция слойя). */
export type CalendarButtonState = 'default' | 'selected' | 'today';

export type CalendarButtonProps = ComponentProps<'button'> & {
  readonly state?: CalendarButtonState;
};

export function CalendarButton({ state = 'default', className, children, ...props }: CalendarButtonProps): JSX.Element {
  return (
    <button
      className={cn(
        'inline-flex h-12 w-12 shrink-0 cursor-pointer items-center justify-center rounded-xl bg-surface font-sans text-base font-medium leading-[18px] text-content outline-none transition-colors',
        'focus-visible:ring-2 focus-visible:ring-primary',
        'disabled:pointer-events-none disabled:opacity-50',
        state === 'default' && 'hover:bg-surface-muted active:bg-surface-muted-hover',
        state === 'selected' && 'bg-primary text-white hover:bg-primary-hover active:bg-primary-active',
        state === 'today' && 'bg-surface-info',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}
