'use client';

import type { JSX } from 'react';
import { SmallArrowDown, SortingSmallBig } from '@/shared/assets/icons';
import { Button, Modal, ModalContent, RadioGroup, RadioGroupItem } from '@/shared/ui/design';
import type { ContactSortOrder } from '../lib/contact-list-model';

/** Опции сортировки книги контактов (макет 1539:85395). */
const SORT_OPTIONS: ReadonlyArray<{ readonly value: ContactSortOrder; readonly label: string }> = [
  { value: 'asc', label: 'Имя от А до Я' },
  { value: 'desc', label: 'Имя от Я до А' },
];

/** Кнопка-пилюля «Имя» над списком: текущее направление сортировки,
 * открывает шит выбора (макет 1527:74139). */
export function ContactsSortButton({ onOpen }: { readonly onOpen: () => void }): JSX.Element {
  return (
    <div className="mx-6">
      <Button
        variant="secondary"
        size="small"
        leadingIcon={<SortingSmallBig />}
        trailingIcon={<SmallArrowDown />}
        onClick={onOpen}
      >
        Имя
      </Button>
    </div>
  );
}

/** Шит «Сортировать» (макет 1539:85395): радио «А до Я» / «Я до А»;
 * выбор применяет сортировку и закрывает шит. */
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
      <ModalContent title="Сортировать">
        <RadioGroup
          value={order}
          onValueChange={(value) => onOrderChange(value as ContactSortOrder)}
          className="gap-2"
        >
          {SORT_OPTIONS.map((option) => (
            <label
              key={option.value}
              className="flex cursor-pointer items-center justify-between rounded-2xl bg-surface-muted px-3 py-1"
            >
              <span className="text-base font-medium text-content">{option.label}</span>
              <RadioGroupItem value={option.value} />
            </label>
          ))}
        </RadioGroup>
      </ModalContent>
    </Modal>
  );
}
