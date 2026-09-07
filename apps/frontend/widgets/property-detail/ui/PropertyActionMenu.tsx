'use client';

import { useMemo, useState } from 'react';
import type { JSX } from 'react';
import { Kebab } from '@/shared/assets/icons';
import type { PropertyStatus } from '@/entities/property';
import { IconButton } from '@/shared/ui/icon-button';
import { Select, type SelectOption } from '@/shared/ui/select';
import styles from './PropertyActionMenu.module.css';

export type PropertyActionMenuProps = {
    readonly status?: PropertyStatus;
    readonly disabled?: boolean;
    readonly onEdit: () => void;
    readonly onToggleMaintenance: () => void;
    readonly onToggleArchive: () => void;
    readonly onDelete: () => void;
    readonly onAccess: () => void;
};

export function PropertyActionMenu({
    status,
    disabled,
    onEdit,
    onToggleMaintenance,
    onToggleArchive,
    onDelete,
    onAccess,
}: PropertyActionMenuProps): JSX.Element {
    const [selectedAction, setSelectedAction] = useState('');

    const options = useMemo<SelectOption[]>(() => {
        const items: SelectOption[] = [];
        if (status && status !== 'archived') {
            items.push({ value: 'edit', label: 'Редактировать объект' });
            items.push({
                value: 'toggleMaintenance',
                label: status === 'maintenance' ? 'Вернуть в работу' : 'На ремонт',
            });
        }
        if (status) {
            items.push({ value: 'access', label: 'Совместный доступ' });
            items.push({
                value: 'toggleArchive',
                label: status === 'archived' ? 'Вернуть из архива' : 'Перевести в архив',
            });
            items.push({ value: 'delete', label: 'Удалить объект' });
        }
        return items;
    }, [status]);

    return (
        <Select
            value={selectedAction}
            options={options}
            dropdownAlign="right"
            dropdownClassName={styles.dropdown}
            onChange={(value) => {
                switch (value) {
                    case 'edit':
                        onEdit();
                        break;
                    case 'access':
                        onAccess();
                        break;
                    case 'toggleMaintenance':
                        onToggleMaintenance();
                        break;
                    case 'toggleArchive':
                        onToggleArchive();
                        break;
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
                    icon={<Kebab />}
                    disabled={disabled}
                    onClick={onClick}
                />
            )}
        />
    );
}
