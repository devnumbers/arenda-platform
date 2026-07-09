'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Icon} from '@/shared/ui/icon';
import {Plus} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import styles from './FinanceCreateOperationButton.module.css';

export function FinanceCreateOperationButton(): JSX.Element {
    return (
        <NextLink
            href={ROUTES.financeCreateOperation}
            className={styles.addButton}
            aria-label="Добавить операцию"
        >
            <Icon size="s">
                <Plus/>
            </Icon>
        </NextLink>
    );
}
