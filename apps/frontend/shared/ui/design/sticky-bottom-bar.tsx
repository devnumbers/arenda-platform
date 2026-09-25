'use client';

import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { SheetDragHandle } from './sheet-drag-handle';
import { useTabBarSuppression } from './tab-bar';

/** Нижняя закреплённая панель дизайн-слоя (Figma 1043:60106): белый «шит»
 * radius 40 сверху со слотом контента (паддинг 24, зазор 16); каноника —
 * без grabber-ручки (Show Grabber: false), проп `dragHandle` оставлен для
 * включения, если канва вернёт ручку. На широких экранах контент — в
 * колонке max-560. Низ уважает safe-area (home indicator). PageContent даёт
 * снизу 136px, чтобы контент не уходил под панель. Пока панель смонтирована,
 * глушит TabBar: экран с нижней кнопкой действия футера не имеет. */
export type StickyBottomBarProps = {
  readonly children: ReactNode;
  readonly dragHandle?: boolean;
  readonly className?: string;
  /** Контент тянется вместе с шитом в планшетном ярусе 561–1023 (кнопка
   * от края до края минус паддинг 24; при переносе desktop-границы на
   * 1024 диапазон расширился автоматически — решение владельца 08.09
   * #561); на ПК ≥1024 и на мобиле ≤560 — как всегда (базовое решение
   * 2026-09-04 для полноэкранного пикера даты). */
  readonly fullWidthContent?: boolean;
};

export function StickyBottomBar({
  children,
  dragHandle = false,
  className,
  fullWidthContent = false,
}: StickyBottomBarProps): JSX.Element {
  useTabBarSuppression();

  return (
    /* ПК ≥1024 (тикет #865): шит начинается на границе сайдбара
       (--sidebar-inset из .screen-layout; внутри полноэкранной поверхности
       переменная обнулена — её бокс уже правее сайдбара). */
    <div
      className={cn(
        'fixed inset-x-0 bottom-0 z-40 rounded-t-sheet bg-surface font-sans desktop:left-[var(--sidebar-inset,0px)]',
        className,
      )}
    >
      {dragHandle && <SheetDragHandle />}
      {/* Колонка контента 560 (max-w-column, тикет #865) по центру на любой
          ширине — как PageContent; fullWidthContent снимает кап в планшетном
          диапазоне. */}
      <div
        className={cn(
          'mx-auto flex w-full max-w-column flex-col gap-4 p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]',
          fullWidthContent && 'max-desktop:min-[561px]:max-w-none',
        )}
      >
        {children}
      </div>
    </div>
  );
}
