'use client';

import type { ComponentProps, JSX } from 'react';
import { Check, SmallArrowDown, SortingBigSmall, SortingSmallBig } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { ChipButton, Modal, ModalContent } from '@/shared/ui/design';
import type { ContactSortOrder } from '../lib/contact-list-model';

/** Опции сортировки книги контактов (макет 1539:85395). */
const SORT_OPTIONS: ReadonlyArray<{ readonly value: ContactSortOrder; readonly label: string }> = [
  { value: 'asc', label: 'Имя от А до Я' },
  { value: 'desc', label: 'Имя от Я до А' },
];

/** Чип сортировки (макет 1527:74139, механика чипа задач 1535:76222):
 * «Имя»; ведущая иконка — направление (возрастание — SortingSmallBig
 * 418:4608, от меньшего к большему; убывание — SortingBigSmall 418:4607,
 * от большего к меньшему), хвостовая стрелка всегда вниз (671:7320).
 * Прокидывает все пропсы кнопки: на десктопе через него Slot триггера
 * Radix-меню передаёт свои обработчики и aria. */
export function ContactsSortChip({
  order,
  ...props
}: {
  readonly order: ContactSortOrder;
} & ComponentProps<'button'>): JSX.Element {
  return (
    <ChipButton
      leadingIcon={order === 'asc' ? <SortingSmallBig /> : <SortingBigSmall />}
      trailingIcon={<SmallArrowDown />}
      {...props}
    >
      Имя
    </ChipButton>
  );
}

/** Шит «Сортировать» (макет 1539:85395, паттерн шита задач 1535:76225):
 * серая метка, опции-ряды с radio-кружками; выбор применяет сортировку
 * и закрывает шит. */
export function ContactsSortSheet({
  open,
  onOpenChange,
  order,
  onOrderChange,
}: {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly order: ContactSortOrder;
  readonly onOrderChange: (order: ContactSortOrder) => void;
}): JSX.Element {
  return (
    <Modal open={open} onOpenChange={onOpenChange}>
      <ModalContent
        title="Сортировать"
        titleClassName="text-base font-medium leading-[18px] text-content-tertiary"
      >
        <div className="flex flex-col gap-2" role="radiogroup" aria-label="Сортировать">
          {SORT_OPTIONS.map((option) => (
            <SortOptionRow
              key={option.value}
              label={option.label}
              selected={order === option.value}
              onSelect={() => onOrderChange(option.value)}
            />
          ))}
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
