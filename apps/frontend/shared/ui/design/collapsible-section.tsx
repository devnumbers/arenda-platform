'use client';

import type { JSX, ReactNode } from 'react';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@radix-ui/react-collapsible';
import { SmallArrowDown } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Сворачиваемая секция дизайн-слоя (карта #497 — «Выполненные N» списка
 * задач, Figma 1531:12784): серая карточка с заголовком H3 и серым
 * счётчиком, справа — стрелка (вниз свёрнута, вверх развёрнута), контент —
 * строки списка. По умолчанию свёрнута (решение владельца: выполненные
 * изначально скрыты). Радикс даёт aria-expanded/aria-controls и
 * анимацию высоты через --radix-collapsible-content-height. */
export type CollapsibleSectionProps = {
  readonly title: ReactNode;
  /** Серый счётчик справа от заголовка («Выполненные 4»). */
  readonly count?: number;
  readonly defaultOpen?: boolean;
  readonly open?: boolean;
  readonly onOpenChange?: (open: boolean) => void;
  /** className обёртки карточки (потребитель даёт mx для полей экрана). */
  readonly className?: string;
  readonly children: ReactNode;
};

export function CollapsibleSection({
  title,
  count,
  defaultOpen = false,
  open,
  onOpenChange,
  className,
  children,
}: CollapsibleSectionProps): JSX.Element {
  return (
    <Collapsible
      defaultOpen={defaultOpen}
      open={open}
      onOpenChange={onOpenChange}
      className={cn('rounded-card bg-surface-muted', className)}
    >
      <CollapsibleTrigger
        className={cn(
          'group flex w-full cursor-pointer items-center justify-between gap-3 px-6 pt-6 text-left font-sans outline-none',
          'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface rounded-card',
          // Свёрнутая карточка балансирует верхний паддинг нижним
          // (Figma 1532:50138: 0 0 24px), развёрнутой баланс даёт контент.
          'not-[[data-state=open]]:pb-6',
        )}
      >
        <h2 className="text-xl font-semibold leading-6 text-content">
          {title}
          {count !== undefined && <span className="text-content-tertiary">{` ${count}`}</span>}
        </h2>
        <SmallArrowDown
          className={cn(
            'shrink-0 text-content-tertiary transition-transform duration-200',
            'group-data-[state=open]:rotate-180',
          )}
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent
        className={cn(
          'overflow-hidden data-[state=open]:animate-[collapsible-down_200ms_var(--dl-ease)] data-[state=closed]:animate-[collapsible-up_150ms_var(--dl-ease)]',
        )}
      >
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}
