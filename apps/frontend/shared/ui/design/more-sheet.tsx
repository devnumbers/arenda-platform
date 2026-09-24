'use client';

import { useEffect, useState, type JSX } from 'react';
import { usePathname } from 'next/navigation';
import { Drawer } from 'vaul';
import { getActiveNavItem } from '@/shared/config/get-active-nav-item';
import { moreSheetNavSections, supportNavSection } from '@/shared/config/navigation';
import { SheetDragHandle } from './sheet-drag-handle';
import { SupportModal } from './support-modal';
import { TabNavLink, TabNavAction } from './tab-bar-row';

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

/** Шит «Еще» мобильного TabBar (Figma 1721:57140, тикет #560). Модель
 * «одного целого» (решение владельца 22.09.2026): лист поднимается
 * ИЗ-ЗА настоящего TabBar и читается с ним как одна поверхность —
 * нижний ряд с дублем табов снесён, бар (z-50, над оверлеем) не
 * темнеет и остаётся живым: «Еще» переключает шит (подсвечен активным),
 * «Объекты»/«Уведомления» ведут на разделы и закрывают шит. Поэтому
 * Drawer.Root не-modal (modal={false}): фокус-трап и aria-hidden не
 * трогают бар; скролл-лок фона на время шита — ручной overflow:hidden
 * (в non-modal ни Radix, ни vaul фон не локают). Высота контента
 * заканчивается над баром: pb = 72px ряда + его safe-area паддинг,
 * белое тело листа продолжается за баром до низа экрана. Ручка 48×4
 * и drag-закрытие vaul сохранены; шестая ячейка — «Поддержка»
 * (#766): шит закрывается, открывается модалка «Связаться с нами».
 * Активность пунктов сетки — из нав-модели (getActiveNavItem).
 *
 * Движение — прерываемая transition-модель в globals.css
 * (.more-sheet-*), тайминги по образцу системных шитов iOS (решение
 * владельца 22.09.2026): пружина без отскока (критическое затухание,
 * UISpringTimingParameters dampingRatio 1.0), выезд 440ms на кривой
 * cubic-bezier(0.32,0.72,0,1) — задокументированный «mimic iOS's
 * Sheet», закрытие 330ms, затемнение гаснет синхронно листу; бар
 * кликабелен в любой фазе анимации (он над оверлеем) — разворот
 * посреди закрывания работает по-настоящему. Контент статичен и не
 * размонтируется по open=false — анимация закрытия живёт (vaul issue
 * #558). */
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
    for (const el of document.querySelectorAll<HTMLElement>('[data-vaul-drawer], .more-sheet-backdrop')) {
      el.style.removeProperty('transform');
      el.style.removeProperty('transition');
      el.style.removeProperty('opacity');
    }
  };

  const handleOpenChange = (next: boolean): void => {
    if (!next) clearDragInlineStyles();
    onOpenChange(next);
  };

  // Non-modal шит не получает скролл-лока ни от Radix, ни от vaul —
  // фон под оверлеем не должен прокручиваться, пока шит открыт.
  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previous;
    };
  }, [open]);

  const openSupport = (): void => {
    handleOpenChange(false);
    setSupportOpen(true);
  };

  return (
    <Drawer.Root
      open={open}
      onOpenChange={handleOpenChange}
      onClose={clearDragInlineStyles}
      modal={false}
      repositionInputs={false}
    >
      <Drawer.Portal>
        {/* Свой бэкдроп вместо Drawer.Overlay: в non-modal vaul рендерит
          * Overlay как null (dist: «if (!modal) return null»), а затемнение
          * нам нужно (решение владельца 22.09.2026). data-state ведём из
          * пропа open — портал живёт, пока доезжает закрытие контента
          * (Presence-страж в globals.css), фейд успевает целиком. Тап по
          * бэкдропу закрывает шит явным onClick: vaul в non-modal сам
          * превентит outside-dismissal Radix (dist: «!modal →
          * e.preventDefault()»), расчёт на interact-outside не работает.
          * pointer-events-auto: Radix-слой шита держит body
          * pointer-events:none — без явного значения бэкдроп «прозрачен»
          * и клик уходит в страницу под ним. */}
        <div
          aria-hidden
          data-state={open ? 'open' : 'closed'}
          className="more-sheet-backdrop pointer-events-auto fixed inset-0 z-40 bg-overlay"
          onClick={() => handleOpenChange(false)}
        />
        <Drawer.Content
          className="more-sheet-drawer pointer-events-auto fixed inset-x-0 bottom-0 z-[45] rounded-t-sheet bg-white pb-[calc(72px+max(12px,env(safe-area-inset-bottom)))] font-sans outline-none"
        >
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
        </Drawer.Content>
      </Drawer.Portal>
      <SupportModal open={supportOpen} onOpenChange={setSupportOpen} />
    </Drawer.Root>
  );
}
