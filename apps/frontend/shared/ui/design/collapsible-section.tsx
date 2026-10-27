'use client';

import type { JSX, ReactNode } from 'react';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@radix-ui/react-collapsible';
import { SmallArrowDown } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Сворачиваемая секция дизайн-слоя (карта #497 — «Выполненные N» списка
 * задач, Figma 1531:12784): серая карточка с заголовком H3 и серым
 * счётчиком, справа — стрелка (вниз свёрнута, вверх развёрнута), контент —
 * строки списка. По умолчанию свёрнута (решение владельца: выполненные
 * изначально скрыты). Радикс даёт aria-expanded/aria-controls; движение —
 * transition grid-rows+opacity (.collapsible-content в globals.css):
 * 300ms --dl-ease в обе стороны, на закрытии opacity 150ms. */
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
          'collapsible-trigger',
          'group flex w-full cursor-pointer items-center justify-between gap-3 px-6 pt-6 text-left font-sans outline-none',
          'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface rounded-card',
          // Свёрнутая карточка балансирует верхний паддинг нижним
          // (Figma 1532:50138: 0 0 24px), развёрнутой баланс даёт контент.
          // Паддинг на transition: мгновенный свап двигал контент на 24px
          // в первом кадре закрытия.
          'not-[[data-state=open]]:pb-6 transition-[padding-bottom] duration-300 ease-apple',
        )}
      >
        <h2 className="text-xl font-semibold leading-6 text-content">
          {title}
          {count !== undefined && <span className="text-content-tertiary">{` ${count}`}</span>}
        </h2>
        <SmallArrowDown
          className={cn(
            'shrink-0 text-content-tertiary transition-transform duration-300 ease-apple',
            'group-data-[state=open]:rotate-180',
          )}
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent className={cn('collapsible-content')}>
        {/* Движение живёт на внутреннем грид-слое: Presence-детектор Radix
            щупает сам CollapsibleContent инлайн-стилями (transition-duration:
            0s, animation-name: none) тем же коммитом, что и смену
            data-state, — transitions на нём не рождаются. Обрезающий див —
            единственный грид-айтем: его overflow-hidden обнуляет
            автоматический минимум, иначе трек 0fr не доезжает до нуля
            (канон tariff-faq: сетка снаружи, обрез внутри). */}
        <div className="collapsible-motion">
          <div className="overflow-hidden">{children}</div>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
