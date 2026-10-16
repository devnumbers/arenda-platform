'use client';

import type { ComponentProps, JSX } from 'react';
import {
  SortingBigSmall,
  SortingSmallBig,
  SmallArrowDown,
} from '@/shared/assets/icons';
import { ChipButton, type PickerMenuGroup } from '@/shared/ui/design';
import type { TasksSort } from '@/features/tasks';

/** Чип сортировки задач (Figma 1535-76222): «Дата»/«Название»; ведущая иконка —
 * направление (возрастание — SortingSmallBig 418:4608, от меньшего к
 * большему; убывание — SortingBigSmall 418:4607, от большего к меньшему —
 * решение владельца 2026-09-03), хвостовая стрелка всегда вниз (671:7320).
 * Прокидывает все пропсы кнопки: триггер PickerMenu через asChild передаёт
 * ему свои обработчики и aria. Общий для объектного экрана (#499) и
 * глобальной ленты (#523). */
export function SortChip({
  sort,
  ...props
}: {
  readonly sort: TasksSort;
} & ComponentProps<'button'>): JSX.Element {
  return (
    <ChipButton
      leadingIcon={
        sort.direction === 'asc' ? <SortingSmallBig /> : <SortingBigSmall />
      }
      trailingIcon={<SmallArrowDown />}
      {...props}
    >
      {sort.field === 'date' ? 'Дата' : 'Название'}
    </ChipButton>
  );
}

/** Группы опций пикера сортировки (Figma 1603-94487/1535-76225): поле и
 * направление — два независимых «радио»; выбор применяет sort сразу. */
export function sortPickerGroups(
  sort: TasksSort,
  onSortChange: (sort: TasksSort) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [
        {
          label: 'По дате создания',
          selected: sort.field === 'date',
          onSelect: () => onSortChange({ ...sort, field: 'date' }),
        },
        {
          label: 'По названию',
          selected: sort.field === 'title',
          onSelect: () => onSortChange({ ...sort, field: 'title' }),
        },
      ],
    },
    {
      options: [
        {
          label: 'Возрастание',
          selected: sort.direction === 'asc',
          onSelect: () => onSortChange({ ...sort, direction: 'asc' }),
        },
        {
          label: 'Убывание',
          selected: sort.direction === 'desc',
          onSelect: () => onSortChange({ ...sort, direction: 'desc' }),
        },
      ],
    },
  ];
}
