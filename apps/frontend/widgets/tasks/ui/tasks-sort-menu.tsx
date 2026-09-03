'use client';

import type { JSX, ReactNode } from 'react';
import { Check } from '@/shared/assets/icons';
import type { TasksSort, TasksSortDirection, TasksSortField } from '@/features/tasks';
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@/shared/ui/design';

/** Меню сортировки списка задач на десктопе (Figma 1603-94487): карточка
 * 246px с двумя группами опций через разделитель — поле («По дате
 * создания» / «По названию») и направление («Возрастание» / «Убывание»);
 * у пункта ведущий selection-квадрат (radius 8): синий с галкой у
 * выбранного, серое кольцо у остальных. Выбор сразу применяет сортировку
 * и закрывает меню (решение владельца 2026-09-03); на мобильной ширине
 * вместо меню остаётся шит TasksSortSheet (Figma 1535-76225). */
export type TasksSortMenuProps = {
  readonly sort: TasksSort;
  readonly onSortChange: (sort: TasksSort) => void;
  readonly children: ReactNode;
};

export function TasksSortMenu({
  sort,
  onSortChange,
  children,
}: TasksSortMenuProps): JSX.Element {
  const pickField = (field: TasksSortField): void => onSortChange({ ...sort, field });
  const pickDirection = (direction: TasksSortDirection): void =>
    onSortChange({ ...sort, direction });

  return (
    <Menu>
      <MenuTrigger asChild>{children}</MenuTrigger>
      <MenuContent align="start" className="w-[246px] gap-0">
        <div className="flex flex-col gap-0.5">
          <SortMenuOption
            label="По дате создания"
            selected={sort.field === 'date'}
            onSelect={() => pickField('date')}
          />
          <SortMenuOption
            label="По названию"
            selected={sort.field === 'title'}
            onSelect={() => pickField('title')}
          />
        </div>
        <div className="mt-3 flex flex-col gap-0.5 border-t border-surface-muted pt-3">
          <SortMenuOption
            label="Возрастание"
            selected={sort.direction === 'asc'}
            onSelect={() => pickDirection('asc')}
          />
          <SortMenuOption
            label="Убывание"
            selected={sort.direction === 'desc'}
            onSelect={() => pickDirection('desc')}
          />
        </div>
      </MenuContent>
    </Menu>
  );
}

/** Пункт меню (Figma 1186:44732 + Selection Button Checkbox, radius 8):
 * подпись 14/500 и квадрат выбора — синий с белой галкой у выбранного. */
function SortMenuOption({
  label,
  selected,
  onSelect,
}: {
  readonly label: string;
  readonly selected: boolean;
  readonly onSelect: () => void;
}): JSX.Element {
  return (
    <MenuItem
      onSelect={onSelect}
      icon={
        selected ? (
          <span className="flex h-5 w-5 items-center justify-center rounded-lg bg-primary text-white">
            <Check className="h-4 w-4" aria-hidden />
          </span>
        ) : (
          <span className="h-5 w-5 rounded-lg border-[1.5px] border-content-tertiary" aria-hidden />
        )
      }
    >
      {label}
    </MenuItem>
  );
}
