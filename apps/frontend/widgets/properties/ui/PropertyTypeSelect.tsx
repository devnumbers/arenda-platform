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
import { propertyTypeOptions } from '@/features/properties/lib/property-types';
import type { PropertyType } from '@/entities/property/model/types';
import { ChevronDown } from '@/shared/assets/icons';
import styles from './PropertyTypeSelect.module.css';

export type PropertyTypeSelectProps = {
  readonly value?: PropertyType;
  readonly onChange: (value: PropertyType) => void;
  readonly error?: string;
};

export function PropertyTypeSelect({ value, onChange, error }: PropertyTypeSelectProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);

  const selectedLabel = useMemo(
    () => propertyTypeOptions.find((option) => option.value === value)?.label ?? '',
    [value]
  );

  const handleSelect = (selectedValue: PropertyType) => {
    onChange(selectedValue);
    setIsOpen(false);
  };

  return (
    <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger>
        <button
          type="button"
          className={clsx(styles.trigger, error && styles.error)}
        >
          <span className={styles.label}>Тип</span>
          <span className={styles.control}>
            <span className={styles.value}>{selectedLabel}</span>
            <span className={styles.chevron} aria-hidden="true">
              <ChevronDown />
            </span>
          </span>
          {error && <span className={styles.errorText}>{error}</span>}
        </button>
      </PopoverTrigger>
      <PopoverContent className={styles.dropdown}>
        <PopoverDialog aria-label="Тип объекта">
          <ListBox
            aria-label="Тип объекта"
            selectionMode="single"
            selectedKeys={value ? new Set([value]) : new Set()}
            onSelectionChange={(keys) => {
              const selectedKey = Array.from(keys)[0];
              const selectedOption = propertyTypeOptions.find(
                (option) => option.value === selectedKey
              );
              if (selectedOption) {
                handleSelect(selectedOption.value);
              }
            }}
          >
            {propertyTypeOptions.map((option) => (
              <ListBoxItem key={option.value}>{option.label}</ListBoxItem>
            ))}
          </ListBox>
        </PopoverDialog>
      </PopoverContent>
    </Popover>
  );
}
