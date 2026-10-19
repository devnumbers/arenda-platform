'use client';

import {
  createContext,
  forwardRef,
  useContext,
  useRef,
  type ComponentProps,
  type JSX,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from 'react';
import {
  Content,
  Item,
  Portal,
  Root,
  Trigger,
} from '@radix-ui/react-dropdown-menu';
import { cn } from '@/shared/lib/cn';
import { useFocusRestore, type FocusRestore } from './focus-restore';

/** Меню-кебаб дизайн-слоя (карта #497, Figma 1535-77633 — меню списка
 * задач) — dropdown на Radix: белая карточка radius 24 с тенью, пункты —
 * кнопки «Button White Small» (Figma 1186:44732): иконка 24 + подпись
 * 14/500, radius 16, hover — серый фон. Позиция — под якорем с
 * выравниванием по правому краю (кебаб у правого края экрана), вылет за
 * вьюпорт гасит collision-детект Радикса. Типобезопасно повторяет API
 * примитива: Menu (Root) / MenuTrigger / MenuContent / MenuItem. */

/** Пара capture/restore одного цикла меню, общая для триггера и
 * контента. DropdownMenu не экспортирует onOpenAutoFocus, а к закрытию
 * активный элемент — уже пункт меню, поэтому хозяин пинится на самом
 * триггере, в момент его события: currentTarget — детерминированный
 * триггер (фокус на него браузер ставит дефолт-действием позже
 * pointerdown'а), а entry-focus внутрь контента Radix делает уже после.
 * Без провайдера (Trigger/Content вне Menu) хендлеры деградируют к
 * прежнему поведению — null-безопасно. */
const MenuFocusRestoreContext = createContext<FocusRestore | null>(null);

export function Menu({ children, ...props }: ComponentProps<typeof Root>): JSX.Element {
  const restore = useFocusRestore();
  return (
    <MenuFocusRestoreContext.Provider value={restore}>
      <Root {...props}>{children}</Root>
    </MenuFocusRestoreContext.Provider>
  );
}

export const MenuTrigger = forwardRef<HTMLButtonElement, ComponentProps<typeof Trigger>>(
  function MenuTrigger({ onPointerDown, onKeyDown, ...props }, forwardedRef) {
    const restore = useContext(MenuFocusRestoreContext);
    return (
      <Trigger
        {...props}
        ref={forwardedRef}
        onPointerDown={(event: ReactPointerEvent<HTMLButtonElement>) => {
          restore?.capture(event.currentTarget);
          onPointerDown?.(event);
        }}
        onKeyDown={(event: ReactKeyboardEvent<HTMLButtonElement>) => {
          restore?.capture(event.currentTarget);
          onKeyDown?.(event);
        }}
      />
    );
  },
);

export type MenuContentProps = ComponentProps<typeof Content>;

export function MenuContent({
  className,
  sideOffset = 4,
  align = 'end',
  onCloseAutoFocus,
  ...props
}: MenuContentProps): JSX.Element {
  const contentRef = useRef<HTMLDivElement>(null);
  const restore = useContext(MenuFocusRestoreContext);
  return (
    <Portal>
      <Content
        ref={contentRef}
        sideOffset={sideOffset}
        align={align}
        className={cn(
          // Figma 1535-77633: карточка radius 24, паддинг 12px 2px, зазор
          // пунктов 2px, тень 0 8px 24px rgba(0,0,0,0.12).
          'z-50 flex w-fit flex-col gap-0.5 rounded-card bg-surface px-0.5 py-3 font-sans shadow-[0_8px_24px_rgba(23,26,28,0.12)] outline-none',
          '[transform-origin:var(--radix-dropdown-menu-content-transform-origin,center)]',
          'data-[state=open]:animate-[menu-in_150ms_var(--dl-ease)]',
          'data-[state=closed]:animate-[menu-out_120ms_var(--dl-ease)]',
          className,
        )}
        // Повторное открытие в окне exit-анимации: слой меню в exit ещё
        // смонтирован (Presence ждёт animationend), и его dismiss на
        // document-уровне срабатывает ПОЗЖЕ toggle'а триггера в том же
        // pointerdown — гася его: быстрый повторный клик по триггеру не
        // открывает меню (поймано e2e сортировок, план 009). pointerdown
        // по триггеру dismiss не запрещаем — его toggle уже отыграл; клик
        // мимо триггера закрывает меню как раньше.
        onPointerDownOutside={(event) => {
          const contentId = contentRef.current?.id;
          const target = event.detail.originalEvent.target;
          if (
            contentId !== undefined &&
            target instanceof Element &&
            target.closest(`[aria-controls="${contentId}"]`) !== null
          ) {
            event.preventDefault();
          }
        }}
        onCloseAutoFocus={(event) => {
          // Потребитель раньше, безкольцевой возврат после: наш
          // preventDefault гасит дефолтный triggerRef.focus() Radix —
          // именно он и рисует залипшее кольцо (контракт «никогда»,
          // решение владельца 19.10).
          onCloseAutoFocus?.(event);
          restore?.restore(event);
        }}
        {...props}
      />
    </Portal>
  );
}

export type MenuItemProps = ComponentProps<typeof Item> & {
  readonly icon?: ReactNode;
  readonly children: ReactNode;
};

export function MenuItem({ className, icon, children, ...props }: MenuItemProps): JSX.Element {
  return (
    <Item
      className={cn(
        'flex h-11 cursor-pointer select-none items-center gap-1.5 rounded-button px-5 text-sm font-medium text-content outline-none transition-colors',
        'data-[highlighted]:bg-surface-muted data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        // Нажатие темнее ховера — сет DesktopMenuButton (невыбранной):
        // hover bg-surface-muted, при нажатии bg-surface-muted-hover.
        // Комбинированный вариант гарантирует шаг поверх data-highlighted
        // на десктопе (одинаковая специфичность пары решила бы порядок
        // stylesheet, а не нажатие); голый active закрывает тач-тап без
        // highlighted.
        'active:bg-surface-muted-hover active:data-[highlighted]:bg-surface-muted-hover',
        className,
      )}
      {...props}
    >
      {icon !== undefined && (
        <span className="flex h-6 w-6 shrink-0 items-center justify-center" aria-hidden>
          {icon}
        </span>
      )}
      {children}
    </Item>
  );
}
