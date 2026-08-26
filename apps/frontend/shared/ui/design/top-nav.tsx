import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

/** Верхняя навигация экрана дизайн-слоя (Figma 934:19656–59): белая полоса
 * высотой 44 со слотами — leading слева (кнопка «назад»), trailing справа
 * (кнопки действий), в центре children (Title+Subtitle, StepsChip, поиск
 * или лого — композиция экрана). Стикится к верху, уважает safe-area.
 *
 * Раскладка — grid `auto minmax(0,1fr) auto`: центр занимает место строго
 * между слотами и сжимается с min-w-0 (truncate у длинных заголовков), не
 * наезжая на них; без слотов колонки схлопываются и центр остаётся по
 * центру полосы. На узких экранах навигация сжимается вместе с вьюпортом,
 * а не распирает страницу (правка 2026-08-26: px-костыль 56×2 держал
 * минимум ~325px и обрезал экраны ~320px). */
export type TopNavProps = {
  readonly leading?: ReactNode;
  readonly trailing?: ReactNode;
  readonly children?: ReactNode;
  readonly className?: string;
};

export function TopNav({ leading, trailing, children, className }: TopNavProps): JSX.Element {
  return (
    <header className={cn('sticky top-0 z-40 bg-white font-sans pt-[env(safe-area-inset-top)]', className)}>
      <div className="relative mx-auto grid h-11 w-full max-w-[560px] grid-cols-[auto_minmax(0,1fr)_auto] items-center">
        {leading !== undefined && <div className="flex items-center pl-3.5">{leading}</div>}
        <div className="flex min-w-0 items-center justify-center gap-2 px-3">{children}</div>
        {trailing !== undefined && <div className="flex items-center pr-3.5">{trailing}</div>}
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
