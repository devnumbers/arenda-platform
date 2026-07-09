'use client';

import { useMemo, useState } from 'react';
import type { JSX } from 'react';
import { Menu } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { IconButton } from '@/shared/ui/icon-button';
import { Select, type SelectOption } from '@/shared/ui/select';
import styles from './OperationActionMenu.module.css';

type OperationStatus = components['schemas']['OperationStatus'];
type OperationType = components['schemas']['OperationType'];

export type OperationActionMenuProps = {
  readonly type?: OperationType;
  readonly status?: OperationStatus;
  readonly disabled?: boolean;
  readonly onComplete: () => void;
  readonly onMarkIncomplete: () => void;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
};

export function OperationActionMenu({
  type,
  status,
  disabled,
  onComplete,
  onMarkIncomplete,
  onEdit,
  onDelete,
}: OperationActionMenuProps): JSX.Element {
  const [selectedAction, setSelectedAction] = useState('');
  const isIncome = type === 'income';

  const options = useMemo<SelectOption[]>(() => {
    if (disabled) return [];
    const items: SelectOption[] = [];
    if (status === 'pending' || status === 'overdue') {
      items.push({ value: 'complete', label: isIncome ? 'Отметить полученной' : 'Отметить оплаченной' });
    }
    if (status === 'paid' || status === 'received') {
      items.push({ value: 'markIncomplete', label: isIncome ? 'Отметить не полученной' : 'Отметить не оплаченной' });
    }
    items.push({ value: 'edit', label: 'Редактировать' });
    items.push({ value: 'delete', label: 'Удалить' });
    return items;
  }, [disabled, status, isIncome]);

  return (
    <Select
      value={selectedAction}
      options={options}
      dropdownAlign="right"
      dropdownClassName={styles.dropdown}
      onChange={(value) => {
        switch (value) {
          case 'complete':
            onComplete();
            break;
          case 'markIncomplete':
            onMarkIncomplete();
            break;
          case 'edit':
            onEdit();
            break;
          case 'delete':
            onDelete();
            break;
        }
        setSelectedAction('');
      }}
      renderTrigger={({ onClick }) => (
        <IconButton
          variant="secondary"
          size="large"
          aria-label="Действия"
          icon={<Menu />}
          disabled={disabled}
          onClick={onClick}
        />
      )}
    />
  );
}
