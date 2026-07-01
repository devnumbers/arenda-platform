'use client';

import type {JSX, ReactNode} from 'react';
import {Popover, PopoverContent, PopoverDialog, PopoverTrigger,} from '@heroui/react/popover';
import {ChevronDown} from '@/shared/assets/icons';
import {Button} from '@/shared/ui/button';
import {Icon} from '@/shared/ui/icon';
import styles from './FilterPopover.module.css';

export type FilterPopoverProps = {
    readonly label: string;
    readonly activeCount: number;
    readonly children: ReactNode;
};

export function FilterPopover({label, activeCount, children}: FilterPopoverProps): JSX.Element {
    const triggerLabel = activeCount > 0 ? `${label} ${activeCount}` : label;

    return (
        <Popover>
            <PopoverTrigger>
                <Button
                    className={styles.trigger}
                    variant="secondary"
                    size="medium"
                    rightIcon={
                        <Icon size="s">
                            <ChevronDown/>
                        </Icon>
                    }
                >
                    {triggerLabel}
                </Button>
            </PopoverTrigger>
            <PopoverContent className={styles.content}>
                <PopoverDialog aria-label={label} className={styles.dialog}>
                    {children}
                </PopoverDialog>
            </PopoverContent>
        </Popover>
    );
}
