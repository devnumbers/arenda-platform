'use client';

import type { JSX } from 'react';
import { usePathname } from 'next/navigation';
import { Drawer } from 'vaul';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { moreSheetNavSections } from '@/shared/config/navigation';
import { SheetDragHandle } from './sheet-drag-handle';
import { TabBarRow, TabNavLink } from './tab-bar-row';

/** Пунктов в ряду шита — три, рядов два (Figma 1721:57140). */
const SHEET_ROW_LENGTH = 3;

export type MoreSheetProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
};

/** Шит «Еще» мобильного TabBar (Figma 1721:57140, тикет #560; рецепт
 * плавного выезда — docs/research/2026-09-08-sheet-eshche-animation.md,
 * тикет #557): белый лист radius 40 сверху на vaul (drag с инерцией,
 * закрытие свайпом вниз), оверлей bg-overlay = rgba(23,26,28,0.5); ручка
 * 48×4; два ряда навигации по три пункта (иконка 24 + подпись 13/15,
 * min-h-72, px-16, зазоры 8) из нав-модели moreSheetNavSections; нижний
 * ряд — тот же TabBarRow («Еще» активен, тап закрывает шит), затем
 * safe-area (home indicator; фолбэк 12px — от iOS-бага env()=0 в
 * standalone-PWA, research §4) — геометрия ряда совпадает с реальным
 * баром, полоса визуально непрерывна, без двойной полосы. Активность
 * пунктов — из нав-модели (getActiveNavItem).
 *
 * Тайминги — в globals.css (.more-sheet-*): выезд 400ms, закрытие 300ms,
 * оверлей — канонный fade 250ms; закрытие — оверлей, Esc (Radix), свайп
 * vaul и повторный тап «Еще» (кнопка нижнего ряда). shouldScaleBackground
 * не включаем (research: чёрные вспышки, #259), snap points не нужны —
 * шит фиксированной навигационной высоты. Контент статичен и не
 * размонтируется по open=false — анимация закрытия живёт (vaul issue
 * #558). Пункты ведут на живые маршруты: «Платежи» и «Участники» —
 * страницы-заглушки (#559). */
export function MoreSheet({ open, onOpenChange }: MoreSheetProps): JSX.Element {
  const pathname = usePathname();
  const activeSectionId = getActiveNavItem(pathname)?.id;

  return (
    <Drawer.Root open={open} onOpenChange={onOpenChange} repositionInputs={false}>
      <Drawer.Portal>
        <Drawer.Overlay className="more-sheet-overlay fixed inset-0 z-50 bg-overlay" />
        <Drawer.Content className="more-sheet-drawer fixed inset-x-0 bottom-0 z-50 rounded-t-sheet bg-white pb-[max(12px,env(safe-area-inset-bottom))] font-sans outline-none">
          <Drawer.Title className="sr-only">Еще</Drawer.Title>
          <SheetDragHandle wide />
          <div className="flex flex-col gap-2 py-2">
            {[0, SHEET_ROW_LENGTH].map((rowStart) => (
              <div key={rowStart} className="flex min-h-[72px] items-stretch px-4">
                {moreSheetNavSections
                  .slice(rowStart, rowStart + SHEET_ROW_LENGTH)
                  .map((section) => (
                    <TabNavLink
                      key={section.id}
                      section={section}
                      active={activeSectionId === section.id}
                      onClick={() => onOpenChange(false)}
                    />
                  ))}
              </div>
            ))}
          </div>
          <nav aria-label="Нижняя навигация">
            <TabBarRow
              moreActive
              onMoreSelect={() => onOpenChange(false)}
              onNavigate={() => onOpenChange(false)}
            />
          </nav>
        </Drawer.Content>
      </Drawer.Portal>
    </Drawer.Root>
  );
}
