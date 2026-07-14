'use client';

import {useMemo, useState} from 'react';
import type {JSX} from 'react';
import {useRouter} from 'next/navigation';
import {Menu} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {IconButton} from '@/shared/ui/icon-button';
import {Select, type SelectOption} from '@/shared/ui/select';
import styles from './LeaseActionMenu.module.css';

export type LeaseActionMenuProps = {
    readonly leaseId: string;
    readonly canEdit: boolean;
    readonly canComplete: boolean;
    readonly onComplete: () => void;
};

export function LeaseActionMenu({
    leaseId,
    canEdit,
    canComplete,
    onComplete,
}: LeaseActionMenuProps): JSX.Element {
    const router = useRouter();
    const [selectedAction, setSelectedAction] = useState('');

    const options = useMemo<SelectOption[]>(() => {
        const items: SelectOption[] = [];
        items.push({value: 'edit', label: 'Редактировать аренду'});
        if (canComplete) {
            items.push({value: 'complete', label: 'Завершить аренду'});
        }
        return items;
    }, [canComplete]);

    return (
        <Select
            label="Действия"
            value={selectedAction}
            options={options}
            dropdownAlign="right"
            dropdownClassName={styles.dropdown}
            onChange={(value) => {
                switch (value) {
                    case 'edit':
                        router.push(ROUTES.leaseEdit(leaseId));
                        break;
                    case 'complete':
                        onComplete();
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
