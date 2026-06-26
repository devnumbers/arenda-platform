'use client';

import { useMemo, useState, type JSX } from 'react';
import clsx from 'clsx';
import {
  Popover,
  PopoverContent,
  PopoverDialog,
  PopoverTrigger,
  ListBox,
  ListBoxItem,
} from '@heroui/react';
import {
  incomeCategories,
  expenseCategories,
  type OperationType,
  type OperationCategory,
} from '@/entities/operation/model/types';
import { ChevronDown } from '@/shared/assets/icons';
import styles from './CategorySelect.module.css';

export type CategorySelectProps = {
  readonly type: OperationType;
  readonly value?: OperationCategory;
  readonly onChange: (category: OperationCategory) => void;
  readonly error?: string;
};

export function CategorySelect({
  type,
  value,
  onChange,
  error,
}: CategorySelectProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);

  const options = useMemo(
    () => (type === 'income' ? incomeCategories : expenseCategories),
    [type]
  );

  const selectedLabel = useMemo(
    () => options.find((option) => option.value === value)?.label ?? '',
    [options, value]
  );

  const handleSelect = (selectedValue: OperationCategory) => {
    onChange(selectedValue);
    setIsOpen(false);
  };

  const label = type === 'income' ? 'Категория дохода' : 'Категория расхода';

  return (
    <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger>
        <button
          type="button"
          className={clsx(styles.trigger, error && styles.error)}
          aria-label={label}
        >
          <span className={styles.label}>{label}</span>
          <span className={styles.control}>
            <span className={clsx(styles.value, !value && styles.placeholder)}>
              {selectedLabel || 'Выберите категорию'}
            </span>
            <span className={styles.chevron} aria-hidden="true">
              <ChevronDown />
            </span>
          </span>
          {error && <span className={styles.errorText}>{error}</span>}
        </button>
      </PopoverTrigger>
      <PopoverContent className={styles.dropdown}>
        <PopoverDialog aria-label={label}>
          <ListBox
            aria-label={label}
            selectionMode="single"
            selectedKeys={value ? new Set([value]) : new Set()}
            onSelectionChange={(keys) => {
              const selectedKey = Array.from(keys)[0];
              const selectedOption = options.find(
                (option) => option.value === selectedKey
              );
              if (selectedOption) {
                handleSelect(selectedOption.value);
              }
            }}
          >
            {options.map((option) => (
              <ListBoxItem key={option.value} className={styles.item}>
                {option.label}
              </ListBoxItem>
            ))}
          </ListBox>
        </PopoverDialog>
      </PopoverContent>
    </Popover>
  );
}
