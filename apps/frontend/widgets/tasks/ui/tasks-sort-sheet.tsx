'use client';

import type { JSX } from 'react';
import { Check } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { Modal, ModalContent } from '@/shared/ui/design';
import type { TasksSort, TasksSortDirection, TasksSortField } from '@/features/tasks';

/** Шит «Сортировать» списка задач (Figma 1535-76225): серая метка, две
 * группы опций-рядов с radio-кружками — поле («По дате создания» /
 * «По названию») и направление («Возрастание» / «Убывание»); выбор сразу
 * применяет сортировку и остаётся в шите до закрытия. Сортировка меняет
 * только порядок строк внутри групп (резолюция #497). */
export type TasksSortSheetProps = {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly sort: TasksSort;
  readonly onSortChange: (sort: TasksSort) => void;
};

export function TasksSortSheet({
  open,
  onOpenChange,
  sort,
  onSortChange,
}: TasksSortSheetProps): JSX.Element {
  const pickField = (field: TasksSortField): void => onSortChange({ ...sort, field });
  const pickDirection = (direction: TasksSortDirection): void =>
    onSortChange({ ...sort, direction });

  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent
        title="Сортировать"
        titleClassName="text-base font-medium leading-[18px] text-content-tertiary"
      >
        <div className="flex flex-col gap-6" role="radiogroup" aria-label="Сортировать">
          <div className="flex flex-col gap-2">
            <SortOptionRow
              label="По дате создания"
              selected={sort.field === 'date'}
              onSelect={() => pickField('date')}
            />
            <SortOptionRow
              label="По названию"
              selected={sort.field === 'title'}
              onSelect={() => pickField('title')}
            />
          </div>
          <div className="flex flex-col gap-2 border-t border-surface-muted pt-6">
            <SortOptionRow
              label="Возрастание"
              selected={sort.direction === 'asc'}
              onSelect={() => pickDirection('asc')}
            />
            <SortOptionRow
              label="Убывание"
              selected={sort.direction === 'desc'}
              onSelect={() => pickDirection('desc')}
            />
          </div>
        </div>
      </ModalContent>
    </Modal>
  );
}

/** Опция-ряд (Figma Row Button 1041:48796): серая плашка 48 radius 16,
 * заголовок 16/18 слева, radio-кружок справа — синий с галкой у выбранной. */
function SortOptionRow({
  label,
  selected,
  onSelect,
}: {
  readonly label: string;
  readonly selected: boolean;
  readonly onSelect: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        'flex h-12 w-full cursor-pointer items-center justify-between gap-3 rounded-button bg-surface-muted pr-3 pl-5 text-left font-sans outline-none transition-colors',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
      )}
    >
      <span className="text-base font-medium text-content">{label}</span>
      {selected ? (
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-pill bg-primary text-white">
          <Check className="h-4 w-4" aria-hidden />
        </span>
      ) : (
        <span className="h-6 w-6 shrink-0 rounded-pill border-[1.5px] border-content-tertiary" aria-hidden />
      )}
    </button>
  );
}
