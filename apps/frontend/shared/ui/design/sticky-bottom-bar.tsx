'use client';

import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { useVisualKeyboardInset } from '@/shared/lib/hooks/useVisualKeyboardInset';
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
 * Кнопка над клавиатурой (#1151, research #1148 §B) — двухслойно: на
 * Android мета interactive-widget=resizes-content (app/layout.tsx) сжимает
 * layout viewport, и fixed-панель поднимается сама; iOS Safari мету
 * игнорирует — там useVisualKeyboardInset сдвигает панель трансформом в
 * координаты visual viewport (композитный сдвиг, без re-layout). Трансформ
 * на самой панели не делает её containing block для fixed потомков —
 * запрет «поверхность — не containing block» (DESIGN.md §1) про предков.
 * На ПК хук бездействует (слушатели не вешаются). */
export type StickyBottomBarProps = {
  readonly children: ReactNode;
  readonly dragHandle?: boolean;
  readonly className?: string;
};

export function StickyBottomBar({
  children,
  dragHandle = false,
  className,
}: StickyBottomBarProps): JSX.Element {
  useTabBarSuppression();
  const keyboardInset = useVisualKeyboardInset();

  return (
    <div
      style={keyboardInset > 0 ? { transform: `translateY(-${keyboardInset}px)` } : undefined}
      className={cn(
        'fixed inset-x-0 bottom-0 z-40 rounded-t-sheet bg-surface font-sans',
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
          в планшетном ярусе кап снят — кнопка во всю ширину шита. */}
      <div className="mx-auto flex w-full max-w-column max-desktop:min-[561px]:max-w-none flex-col gap-4 p-6 pb-[max(1.5rem,env(safe-area-inset-bottom))]">
        {children}
      </div>
    </div>
  );
}
