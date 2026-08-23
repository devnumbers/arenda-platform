'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Card} from '@heroui/react/card';
import {Icon} from '@/shared/ui/icon';
import {Button} from '@/shared/ui/button';
import {PageHeader} from '@/shared/ui/page-header';
import {ArrowRight} from '@/shared/assets/icons';
import {useMe} from '@/features/auth';
import {ROUTES} from '@/shared/config/routes';
import styles from './AccountOverview.module.css';

function AccountOverviewSkeleton(): JSX.Element {
    return (
        <div className={styles.section}>
            <div className={styles.skeletonRow}/>
        </div>
    );
}

export function AccountOverview(): JSX.Element {
    const {data: me, isPending, isError, refetch} = useMe();

    return (
        <>
            <PageHeader title="Аккаунт" backHref={ROUTES.profile}/>
            {isError && (
                <div className={styles.error}>
                    <p className={styles.errorText}>Не удалось загрузить данные</p>
                    <Button onClick={() => void refetch()} variant="secondary">
                        Повторить
                    </Button>
                </div>
            )}
            {!isError && isPending && <AccountOverviewSkeleton/>}
            {!isError && !isPending && (
                <section className={styles.section}>
                    <NextLink
                        href={ROUTES.profileChangePhone}
                        className={styles.phoneLink}
                        prefetch={false}
                    >
                        <Card className={styles.phoneCard}>
                            <div className={styles.phoneLeft}>
                                <span className={styles.phoneLabel}>Телефон</span>
                            </div>
                            <div className={styles.phoneRight}>
                                <span className={styles.phoneValue}>{me.phone}</span>
                                <Icon size="s">
                                    <ArrowRight/>
                                </Icon>
                            </div>
                        </Card>
                    </NextLink>
                </section>
            )}
        </>
    );
}
