'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Tooltip} from '@heroui/react';
import {Icon} from '@/shared/ui/icon';
import {HomeAdd} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import styles from './PropertyCreateButton.module.css';

export type PropertyCreateButtonProps = {
    readonly canAdd: boolean;
};

export function PropertyCreateButton({canAdd}: PropertyCreateButtonProps): JSX.Element {
    if (canAdd) {
        return (
            <NextLink
                href={ROUTES.propertyNew}
                className={styles.addButton}
                aria-label="Добавить объект"
            >
                <Icon size="l">
                    <HomeAdd/>
                </Icon>
            </NextLink>
        );
    }

    return (
        <Tooltip>
            <Tooltip.Trigger>
                <NextLink
                    href={ROUTES.profileTariffChange}
                    className={styles.addButton}
                    aria-label="Сменить тариф"
                >
                    <Icon size="m">
                        <HomeAdd/>
                    </Icon>
                </NextLink>
            </Tooltip.Trigger>
            <Tooltip.Content>
                Достигнут лимит объектов по тарифу
            </Tooltip.Content>
        </Tooltip>
    );
}
