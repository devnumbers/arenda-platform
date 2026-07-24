'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Icon} from '@/shared/ui/icon';
import {ArrowRight} from '@/shared/assets/icons';
import styles from './SectionHeader.module.css';

export type SectionHeaderProps = {
    readonly title: string;
    readonly count?: number;
    readonly href?: string;
};

export function SectionHeader({title, count, href}: SectionHeaderProps): JSX.Element {
    if (!href) {
        return (
            <div className={styles.root}>
                <h2 className={styles.title}>
                    {title}
                    {count !== undefined && <span className={styles.count}> {count}</span>}
                </h2>
            </div>
        );
    }

    return (
        <NextLink href={href} className={styles.root}>
            <h2 className={styles.title}>
                {title}
                {count !== undefined && <span className={styles.count}> {count}</span>}
            </h2>
            <div className={styles.link} aria-label={`Перейти в раздел «${title}»`}>
                <Icon size="s">
                    <ArrowRight/>
                </Icon>
            </div>
        </NextLink>
    );
}
