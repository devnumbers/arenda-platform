'use client';

import type { ComponentProps, JSX, ReactNode } from 'react';
import {
  Content,
  Item,
  Portal,
  Root,
  Trigger,
} from '@radix-ui/react-dropdown-menu';
import { cn } from '@/shared/lib/cn';

/** Меню-кебаб дизайн-слоя (карта #497, Figma 1535-77633 — меню списка
 * задач) — dropdown на Radix: белая карточка radius 24 с тенью, пункты —
 * кнопки «Button White Small» (Figma 1186:44732): иконка 24 + подпись
 * 14/500, radius 16, hover — серый фон. Позиция — под якорем с
 * выравниванием по правому краю (кебаб у правого края экрана), вылет за
 * вьюпорт гасит collision-детект Радикса. Типобезопасно повторяет API
 * примитива: Menu (Root) / MenuTrigger / MenuContent / MenuItem. */
export const Menu = Root;
export const MenuTrigger = Trigger;

export type MenuContentProps = ComponentProps<typeof Content>;

export function MenuContent({ className, sideOffset = 4, align = 'end', ...props }: MenuContentProps): JSX.Element {
  return (
    <Portal>
      <Content
        sideOffset={sideOffset}
        align={align}
        className={cn(
          // Figma 1535-77633: карточка radius 24, паддинг 12px 2px, зазор
          // пунктов 2px, тень 0 8px 24px rgba(0,0,0,0.12).
          'z-50 flex w-fit flex-col gap-0.5 rounded-card bg-surface px-0.5 py-3 font-sans shadow-[0_8px_24px_rgba(23,26,28,0.12)] outline-none',
          'data-[state=open]:animate-[modal-card-in_150ms_var(--dl-ease)]',
          className,
        )}
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
