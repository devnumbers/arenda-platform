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

/** Меню-кебаб дизайн-слоя (карта #497; кебаб ⋮ списка задач, Figma
 * 1535-75894) — dropdown на Radix: белый поповер radius 16 с тенью,
 * пункты 44 с серым hover, опасный пункт — красный текст. Позиция —
 * под якорем с выравниванием по правому краю (кебаб у правого края
 * экрана), вылет за вьюпорт гасит collision-детект Радикса.
 * Типобезопасно повторяет API примитива: Menu (Root) / MenuTrigger /
 * MenuContent / MenuItem. */
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
          'z-50 min-w-[220px] rounded-button bg-surface p-1.5 font-sans shadow-[0_8px_24px_rgba(23,26,28,0.12)] outline-none',
          'data-[state=open]:animate-[modal-card-in_150ms_var(--dl-ease)]',
          className,
        )}
        {...props}
      />
    </Portal>
  );
}

export type MenuItemProps = ComponentProps<typeof Item> & {
  /** Опасный пункт (красный текст) — «Удалить все выполненные». */
  readonly danger?: boolean;
  readonly children: ReactNode;
};

export function MenuItem({ className, danger = false, children, ...props }: MenuItemProps): JSX.Element {
  return (
    <Item
      className={cn(
        'flex h-11 cursor-pointer select-none items-center rounded-[12px] px-4 text-base font-medium outline-none transition-colors',
        danger
          ? 'text-danger data-[highlighted]:bg-surface-danger'
          : 'text-content data-[highlighted]:bg-surface-muted',
        'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        className,
      )}
      {...props}
    >
      {children}
    </Item>
  );
}
