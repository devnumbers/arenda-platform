'use client';

import {type JSX, useState} from 'react';
import {Popover, PopoverContent, PopoverDialog, PopoverTrigger,} from '@heroui/react';
import {IconButton} from '@/shared/ui/icon-button';
import {Menu} from '@/shared/assets/icons';
import type {PropertyStatus} from '@/entities/property/model/types';
import styles from './PropertyActionMenu.module.css';

export type PropertyActionMenuProps = {
    readonly status: PropertyStatus;
    readonly onEdit: () => void;
    readonly onToggleMaintenance: () => void;
    readonly onToggleArchive: () => void;
};

export function PropertyActionMenu({
                                       status,
                                       onEdit,
                                       onToggleMaintenance,
                                       onToggleArchive,
                                   }: PropertyActionMenuProps): JSX.Element {
    const [isOpen, setIsOpen] = useState(false);

    const handleAction = (action: () => void) => {
        action();
        setIsOpen(false);
    };

    return (
        <Popover isOpen={isOpen} onOpenChange={setIsOpen}>
            <PopoverTrigger>
                <IconButton variant={'secondary'} size={'large'}  aria-label="Действия" icon={<Menu/>}/>
            </PopoverTrigger>
            <PopoverContent
                placement="bottom end"
                offset={8}
                className={styles.menu}
            >
                <PopoverDialog aria-label="Действия с объектом" className={styles.dialog}>
                    {status !== 'archived' && (
                        <button
                            type="button"
                            className={styles.item}
                            onClick={() => handleAction(onEdit)}
                        >
                            Редактировать объект
                        </button>
                    )}
                    {status !== 'archived' && (
                        <button
                            type="button"
                            className={styles.item}
                            onClick={() => handleAction(onToggleMaintenance)}
                        >
                            {status === 'maintenance' ? 'Вернуть в работу' : 'На ремонт'}
                        </button>
                    )}
                    <button
                        type="button"
                        className={styles.item}
                        onClick={() => handleAction(onToggleArchive)}
                    >
                        {status === 'archived' ? 'Вернуть из архива' : 'Перевести в архив'}
                    </button>
                </PopoverDialog>
            </PopoverContent>
        </Popover>
    );
}
