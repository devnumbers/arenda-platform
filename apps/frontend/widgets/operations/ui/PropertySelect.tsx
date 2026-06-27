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
import { useProperties } from '@/features/properties/api';
import { ChevronDown } from '@/shared/assets/icons';
import styles from './PropertySelect.module.css';

export type PropertySelectProps = {
  readonly value?: string;
  readonly onChange: (propertyId: string) => void;
  readonly error?: string;
  readonly disabled?: boolean;
};

export function PropertySelect({
  value,
  onChange,
  error,
  disabled,
}: PropertySelectProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(false);
  const { data: properties, isLoading } = useProperties();

  const selectedProperty = useMemo(
    () => properties?.find((property) => property.id === value),
    [properties, value]
  );

  const selectedLabel = selectedProperty?.name ?? '';

  const handleSelect = (selectedValue: string) => {
    onChange(selectedValue);
    setIsOpen(false);
  };

  const isEmpty = !isLoading && properties?.length === 0;
  const isDisabled = isLoading || isEmpty || disabled;

  return (
    <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger>
        <button
          type="button"
          className={clsx(styles.trigger, error && styles.error)}
          disabled={isDisabled}
          aria-label="Объект"
        >
          <span className={styles.label}>Объект</span>
          <span className={styles.control}>
            <span className={clsx(styles.value, !value && styles.placeholder)}>
              {isLoading
                ? 'Загрузка объектов...'
                : selectedLabel || 'Выберите объект'}
            </span>
            <span className={styles.chevron} aria-hidden="true">
              <ChevronDown />
            </span>
          </span>
          {error && <span className={styles.errorText}>{error}</span>}
        </button>
      </PopoverTrigger>
      <PopoverContent className={styles.dropdown}>
        <PopoverDialog aria-label="Объект">
          {isLoading ? (
            <div className={styles.loadingState}>Загрузка объектов...</div>
          ) : isEmpty ? (
            <div className={styles.emptyState}>
              Сначала добавьте объект
            </div>
          ) : (
            <ListBox
              aria-label="Объект"
              selectionMode="single"
              selectedKeys={value ? new Set([value]) : new Set()}
              onSelectionChange={(keys) => {
                const selectedKey = Array.from(keys)[0];
                if (typeof selectedKey === 'string') {
                  handleSelect(selectedKey);
                }
              }}
            >
              {properties?.map((property) => (
                <ListBoxItem
                  key={property.id}
                  className={styles.item}
                  textValue={property.name}
                >
                  <span className={styles.itemContent}>
                    <span className={styles.itemName}>{property.name}</span>
                    {property.address && (
                      <span className={styles.itemAddress}>
                        {property.address}
                      </span>
                    )}
                  </span>
                </ListBoxItem>
              ))}
            </ListBox>
          )}
        </PopoverDialog>
      </PopoverContent>
    </Popover>
  );
}
