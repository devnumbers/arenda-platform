'use client';

import {useState} from 'react';
import type {JSX} from 'react';
import {useRouter} from 'next/navigation';
import {Menu} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {IconButton} from '@/shared/ui/icon-button';
import {Select, type SelectOption} from '@/shared/ui/select';
import styles from './TenantActionMenu.module.css';

export type TenantActionMenuProps = {
    readonly tenantId: string;
    readonly canEdit: boolean;
};

export function TenantActionMenu({
    tenantId,
    canEdit,
}: TenantActionMenuProps): JSX.Element {
    const router = useRouter();
    const [selectedAction, setSelectedAction] = useState('');

    const options: SelectOption[] = [
        {value: 'edit', label: 'Редактировать арендатора'},
    ];

    return (
        <Select
            label="Действия"
            value={selectedAction}
            options={options}
            dropdownAlign="right"
            dropdownClassName={styles.dropdown}
            onChange={(value) => {
                if (value === 'edit') {
                    router.push(ROUTES.tenantEdit(tenantId));
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
