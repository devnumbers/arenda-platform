'use client';

import {useMemo, useState} from 'react';
import type {JSX} from 'react';
import {Menu} from '@/shared/assets/icons';
import {IconButton} from '@/shared/ui/icon-button';
import {Select, type SelectOption} from '@/shared/ui/select';
import styles from './LeaseActionMenu.module.css';

export type LeaseActionMenuProps = {
    readonly isEditing: boolean;
    readonly canEdit: boolean;
    readonly canComplete: boolean;
    readonly canReturnDeposit: boolean;
    readonly onEdit: () => void;
    readonly onComplete: () => void;
    readonly onReturnDeposit: () => void;
};

export function LeaseActionMenu({
    isEditing,
    canEdit,
    canComplete,
    canReturnDeposit,
    onEdit,
    onComplete,
    onReturnDeposit,
}: LeaseActionMenuProps): JSX.Element {
    const [selectedAction, setSelectedAction] = useState('');

    const options = useMemo<SelectOption[]>(() => {
        const items: SelectOption[] = [];
        items.push({
            value: 'edit',
            label: isEditing ? 'Отменить редактирование' : 'Редактировать аренду',
        });
        if (canComplete) {
            items.push({value: 'complete', label: 'Завершить аренду'});
        }
        if (canReturnDeposit) {
            items.push({value: 'returnDeposit', label: 'Вернуть залог'});
        }
        return items;
    }, [isEditing, canComplete, canReturnDeposit]);

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
                    case 'complete':
                        onComplete();
                        break;
                    case 'returnDeposit':
                        onReturnDeposit();
                        break;
                }
                setSelectedAction('');
            }}
            renderTrigger={({onClick}) => (
                <IconButton
                    variant="secondary"
                    size="large"
                    aria-label="Действия"
                    icon={<Menu/>}
                    disabled={!canEdit}
                    onClick={onClick}
                />
            )}
        />
    );
}
