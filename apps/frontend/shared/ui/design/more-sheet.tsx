'use client';

import { useState, type JSX } from 'react';
import { usePathname } from 'next/navigation';
import { Drawer } from 'vaul';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { moreSheetNavSections, supportNavSection } from '@/shared/config/navigation';
import { SheetDragHandle } from './sheet-drag-handle';
import { SupportModal } from './support-modal';
import { TabBarRow, TabNavLink, TabNavAction } from './tab-bar-row';

/** Пунктов в ряду шита — три, рядов два (Figma 1721:57140). */
const SHEET_ROW_LENGTH = 3;

/** Ячейки сетки шита: пять разделов-ссылок, шестая — «Поддержка»-действие
 * (#766): страница /support снесена, ячейка открывает модалку. */
type MoreSheetCell =
  | { readonly kind: 'link'; readonly section: (typeof moreSheetNavSections)[number] }
  | { readonly kind: 'support' };

const moreSheetCells: ReadonlyArray<MoreSheetCell> = [
  ...moreSheetNavSections.map((section) => ({ kind: 'link' as const, section })),
  { kind: 'support' },
];

export type MoreSheetProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
};

/** Шит «Еще» мобильного TabBar (Figma 1721:57140, тикет #560; рецепт
 * плавного выезда — docs/research/2026-09-08-sheet-eshche-animation.md,
 * тикет #557): белый лист radius 40 сверху на vaul (drag с инерцией,
 * закрытие свайпом вниз), оверлей bg-overlay = rgba(23,26,28,0.5); ручка
 * 48×4; два ряда навигации по три пункта (иконка 24 + подпись 13/15,
 * min-h-72, px-16, зазоры 8) из нав-модели moreSheetNavSections, шестая
 * ячейка — «Поддержка»-действие (supportNavSection): шит закрывается,
 * открывается модалка «Связаться с нами» (#766); нижний
 * ряд — тот же TabBarRow («Еще» активен, тап закрывает шит), затем
 * safe-area (home indicator; фолбэк 12px — от iOS-бага env()=0 в
 * standalone-PWA, research §4) — геометрия ряда совпадает с реальным
 * баром, полоса визуально непрерывна, без двойной полосы. Активность
 * пунктов — из нав-модели (getActiveNavItem).
 *
 * Движение — прерываемая transition-модель в globals.css (.more-sheet-*):
 * выезд 300ms, закрытие 240ms из текущей позиции (повторный тап «Еще»
 * посреди закрывания разворачивает шит, без скачков; research
 * docs/research/2026-09-22-bottom-sheet-motion-best-practices.md).
 * Закрытие — оверлей, Esc (Radix), свайп vaul и повторный тап «Еще»
 * (кнопка нижнего ряда). shouldScaleBackground не включаем (research:
 * чёрные вспышки, #259), snap points не нужны — шит фиксированной
 * навигационной высоты. Контент статичен и не размонтируется по
 * open=false — анимация закрытия живёт (vaul issue #558). Пункты ведут
 * на живые маршруты: «Платежи» и «Участники» — страницы-заглушки (#559). */
export function MoreSheet({ open, onOpenChange }: MoreSheetProps): JSX.Element {
  const pathname = usePathname();
  const [supportOpen, setSupportOpen] = useState(false);
  const activeSectionId = getActiveNavItem(pathname)?.id;

  // vaul во время drag ведёт шит inline-стилями (transition:none + текущий
  // transform) и оставляет их висеть после release; transition-модель CSS
  // движением управляет только без inline-хвостов. На старте закрытия
  // сносим их ДО смены data-state: transition стартует из фактической
  // позиции свайпа (лист доезжает оттуда, где его отпустили), а разворот
  // посреди закрывания работает из любой промежуточной точки.
  const clearDragInlineStyles = (): void => {
    for (const el of document.querySelectorAll<HTMLElement>('[data-vaul-drawer], [data-vaul-overlay]')) {
      el.style.removeProperty('transform');
      el.style.removeProperty('transition');
      el.style.removeProperty('opacity');
    }
  };

  const handleOpenChange = (next: boolean): void => {
    if (!next) clearDragInlineStyles();
    onOpenChange(next);
  };

  const openSupport = (): void => {
    handleOpenChange(false);
    setSupportOpen(true);
  };

  return (
    <Drawer.Root open={open} onOpenChange={handleOpenChange} onClose={clearDragInlineStyles} repositionInputs={false}>
      <Drawer.Portal>
        <Drawer.Overlay className="more-sheet-overlay fixed inset-0 z-50 bg-overlay" />
        <Drawer.Content className="more-sheet-drawer fixed inset-x-0 bottom-0 z-50 rounded-t-sheet bg-white pb-[max(12px,env(safe-area-inset-bottom))] font-sans outline-none">
          <Drawer.Title className="sr-only">Еще</Drawer.Title>
          <SheetDragHandle wide />
          <div className="flex flex-col gap-2 py-2">
            {[0, SHEET_ROW_LENGTH].map((rowStart) => (
              <div key={rowStart} className="flex min-h-[72px] items-stretch px-4">
                {moreSheetCells
                  .slice(rowStart, rowStart + SHEET_ROW_LENGTH)
                  .map((cell) =>
                    cell.kind === 'link' ? (
                      <TabNavLink
                        key={cell.section.id}
                        section={cell.section}
                        active={activeSectionId === cell.section.id}
                        onClick={() => handleOpenChange(false)}
                      />
                    ) : (
                      <TabNavAction
                        key={supportNavSection.id}
                        section={supportNavSection}
                        onClick={openSupport}
                      />
                    ),
                  )}
              </div>
            ))}
          </div>
          <nav aria-label="Нижняя навигация">
            <TabBarRow
              moreActive
              onMoreSelect={() => handleOpenChange(false)}
              onNavigate={() => handleOpenChange(false)}
            />
          </nav>
        </Drawer.Content>
      </Drawer.Portal>
      <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
    </Drawer.Root>
  );
}
