import type { ComponentProps, JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Кнопка профиля дизайн-слоя (Figma 699:8867, «User Button»): имя
 * (M/500 14/16, #171A1C → hover #9FA8AC → active #6F787C) и справа
 * аватар-плейсхолдер — круг 44×44 на #F3F4F6 с белым гало 2.5px и глифом
 * Bold/User 24×24 (заливка #D3D7D9 запечена в SVG — это тон плейсхолдера,
 * а не контекстный цвет). Focus-visible — обводка 2px #2B7FFF, только с
 * клавиатуры. Полный «Category Icon» с бейджами (Check/Danger/точка) —
 * отдельный компонент набора 651:5925, сюда не входит. */
export type UserButtonProps = ComponentProps<'button'> & {
  readonly name: string;
};

export function UserButton({ name, className, ...props }: UserButtonProps): JSX.Element {
  return (
    <button
      className={cn(
        'inline-flex cursor-pointer items-center gap-3 rounded-pill px-[14px] font-sans text-sm font-medium leading-4 text-content outline-none transition-colors',
        'hover:text-content-tertiary active:text-content-secondary',
        'focus-visible:ring-2 focus-visible:ring-primary',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    >
      {name}
      <span
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-pill bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
        aria-hidden
      >
        <BoldUser className="h-6 w-6" />
      </span>
    </button>
  );
}
