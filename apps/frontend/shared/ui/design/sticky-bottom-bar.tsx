'use client';

import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { SheetDragHandle } from './sheet-drag-handle';
import { useTabBarSuppression } from './tab-bar';

/** Нижняя закреплённая панель дизайн-слоя (Figma 1043:60106): белый «шит»
 * radius 40 сверху со слотом контента (паддинг 24, зазор 16); каноника —
 * без grabber-ручки (Show Grabber: false), проп `dragHandle` оставлен для
 * включения, если канва вернёт ручку. Контент — в колонке max-560 на мобайле
 * и ПК; на планшете 561–1023 тянется с шитом во всю ширину (кнопка от края
 * до края минус паддинг 24 — решение владельца 28.09, правка после аудита
 * #877: правило универсальное, прежний opt-in `fullWidthContent` пикера дат
 * и визардов аренды снесён). Низ уважает safe-area (home indicator).
 * PageContent даёт снизу 136px, чтобы контент не уходил под панель. Пока
 * панель смонтирована, глушит TabBar: экран с нижней кнопкой действия футера
 * не имеет (мобайл и планшет; на ПК TabBar скрыт всегда).
 *
 * Паттерн «CTA над TabBar» (решение владельца 06.10.2026, #1166) —
 * осознанное отклонение для раздела участников/истории: на этих страницах
 * нижнее меню остаётся видимым, панель поднимается на его высоту (ряд 72 +
 * нижний паддинг бара), `aboveTabBar`. Внутри фуллскрин-поверхностей
 * (пикер объектов приглашения) паттерн не применяется — меню там и так
 * не видно, бар каноничен. Клиренс контента — PageContent `aboveTabBarFooter`. */
export type StickyBottomBarProps = {
  readonly children: ReactNode;
  readonly dragHandle?: boolean;
  readonly className?: string;
  /** Плавать над видимым TabBar ниже ПК, не глуша его (#1166); на ПК
   * TabBar скрыт — панель у нижнего края, как в каноне. */
  readonly aboveTabBar?: boolean;
};

export function StickyBottomBar({
  children,
  dragHandle = false,
  className,
  aboveTabBar = false,
}: StickyBottomBarProps): JSX.Element {
  useTabBarSuppression(!aboveTabBar);

  return (
    <div
      className={cn(
        'fixed inset-x-0 z-40 rounded-t-sheet bg-surface font-sans',
        // Оффсет над футером — только ниже ПК: ряд табов 72 + его нижний
        // паддинг (тот же расчёт, что у history-чипа #718); на ПК TabBar
        // скрыт — панель остаётся у нижнего края.
        aboveTabBar
          ? 'bottom-0 max-desktop:bottom-[calc(72px_+_max(12px,env(safe-area-inset-bottom)))]'
          : 'bottom-0',
        // «Хром ПК постоянен» (решение владельца 25.09, правка канона
        // #561, тикет #865): шит панели на ПК — колонка 560 по центру
        // (left/right уже нулевые, margin auto центрирует при capped
        // ширине), углы снизу свободны — пилюли «Уведомления/Поддержка»
        // не глушатся и кликабельны на любом экране.
        'desktop:mx-auto desktop:max-w-column',
        className,
      )}
    >
      {dragHandle && <SheetDragHandle />}
      {/* Колонка контента 560 (max-w-column, тикет #865) на мобайле и ПК;
          в планшетном ярусе кап снят — кнопка во всю ширину шита. Нижний
          safe-area уважает сам TabBar, под которым панель плавает —
          в режиме aboveTabBar обычный паддинг 24. */}
      <div
        className={cn(
          'mx-auto flex w-full max-w-column max-desktop:min-[561px]:max-w-none flex-col gap-4 p-6',
          !aboveTabBar && 'pb-[max(1.5rem,env(safe-area-inset-bottom))]',
        )}
      >
        {children}
      </div>
    </div>
  );
}
