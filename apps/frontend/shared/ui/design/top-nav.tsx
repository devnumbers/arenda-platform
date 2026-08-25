import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

/** Верхняя навигация экрана дизайн-слоя (Figma 934:19656–59): белая полоса
 * высотой 44 со слотами — leading слева (кнопка «назад»), trailing справа
 * (кнопки действий), в центре children (Title+Subtitle, StepsChip, поиск
 * или лого — композиция экрана). Стикится к верху, уважает safe-area. */
export type TopNavProps = {
  readonly leading?: ReactNode;
  readonly trailing?: ReactNode;
  readonly children?: ReactNode;
  readonly className?: string;
};

export function TopNav({ leading, trailing, children, className }: TopNavProps): JSX.Element {
  return (
    <header className={cn('sticky top-0 z-40 bg-white font-sans pt-[env(safe-area-inset-top)]', className)}>
      <div className="relative mx-auto flex h-11 w-full max-w-[560px] items-center justify-center">
        {leading !== undefined && (
          <div className="absolute left-0 flex items-center pl-3.5">{leading}</div>
        )}
        <div className="flex min-w-0 items-center justify-center gap-2 px-14">{children}</div>
        {trailing !== undefined && (
          <div className="absolute right-0 flex items-center pr-3.5">{trailing}</div>
        )}
      </div>
    </header>
  );
}

/** Центральный блок TopNav варианта Title (Figma 934:19658): заголовок
 * 16/18 + необязательный подзаголовок 14/16 #6F787C. */
export type TopNavTitleProps = {
  readonly title: ReactNode;
  readonly subtitle?: ReactNode;
  readonly className?: string;
};

export function TopNavTitle({ title, subtitle, className }: TopNavTitleProps): JSX.Element {
  return (
    <span className={cn('flex min-w-0 flex-col items-center gap-0.5 text-center', className)}>
      <span className="truncate text-base font-medium text-content">{title}</span>
      {subtitle !== undefined && <span className="text-sm text-content-secondary">{subtitle}</span>}
    </span>
  );
}
