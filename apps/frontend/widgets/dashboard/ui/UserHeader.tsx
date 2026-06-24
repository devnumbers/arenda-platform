'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Icon} from '@/shared/ui/icon';
import {ArrowRight, StarColored} from '@/shared/assets/icons';
import styles from './UserHeader.module.css';

export type UserHeaderProps = {
    readonly name?: string | null;
    readonly tariff?: 'basic' | 'pro' | 'business' | string | null;
};

const tariffLabels: Record<string, string> = {
    basic: 'Базовый',
    pro: 'Pro',
    business: 'Бизнес',
};

export function UserHeader({name, tariff}: UserHeaderProps): JSX.Element {
    const tariffName = tariff ? (tariffLabels[tariff] ?? tariff) : 'Без подписки';

    return (
        <NextLink href="/profile" className={styles.link}>
            <Card className={styles.card}>
                <div className={styles.left}>
                    <span className={styles.name}>{name || 'Пользователь'}</span>
                    <Icon size="s">
                        <ArrowRight/>
                    </Icon>
                </div>
                <span className={styles.badge}>
                  <Icon size="s">
                    <StarColored/>
                  </Icon>
                  <span className={styles.badgeLabel}>{tariffName}</span>
                </span>
            </Card>
        </NextLink>
    );
}
