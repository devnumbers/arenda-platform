'use client';

import type { JSX, ReactNode } from 'react';
import { Search } from '@/shared/assets/icons';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { cn } from '@/shared/lib/cn';

export type SearchPillProps = {
  /** Открытие поисковой страницы хаба: клик и клавиатура (Enter/Space)
   * делают одно и то же действие. */
  readonly onOpenSearch: () => void;
  /** Подпись пилюли: «Найти платёж», «Найти операцию», «Найти контакт». */
  readonly label: string;
  /** Хвост пилюли (декор-слайдеры, «+» создания). Кнопки хвоста глушат
   * всплытие клика сами (stopPropagation): keydown из сфокусированного
   * вложенного элемента строку не активирует (#831). */
  readonly trailing?: ReactNode;
  /** Тестовый хук (data-testid) — только там, где нужен e2e. */
  readonly testId?: string;
  /** Надстройка над канонной строкой классов: pr-хвосты потребителей;
   * сливается через tailwind-merge, победа потребителя. */
  readonly className?: string;
};

/** Пилюля поиска хаба — канон Search Button (DESIGN.md §6): серый
 * rounded-pill на всю ширину, лупа слева, подпись, опциональный хвост.
 * Кликается вся пилюля целиком и активируется с клавиатуры: строка —
 * div с role=button, потому что хвост несёт собственные кнопки, а
 * вложенные кнопки в HTML невалидны (паттерн useKeyboardActivation). */
export function SearchPill({
  onOpenSearch,
  label,
  trailing,
  testId,
  className,
}: SearchPillProps): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect: onOpenSearch });

  return (
    <div
      {...activatorProps}
      data-testid={testId}
      className={cn(
        'flex h-14 w-full cursor-pointer items-center rounded-pill bg-surface-muted pl-[18px] text-left outline-none transition-opacity hover:opacity-90 focus-visible:ring-4 focus-visible:ring-primary active:opacity-90',
        className,
      )}
    >
      <Search className="h-6 w-6 shrink-0 text-content" aria-hidden />
      <span className="min-w-0 flex-1 truncate px-2 text-base font-medium text-content">
        {label}
      </span>
      {trailing}
    </div>
  );
}
