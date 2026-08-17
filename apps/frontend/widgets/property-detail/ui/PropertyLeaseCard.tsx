'use client';

import type {JSX} from 'react';
import NextLink from 'next/link';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {ROUTES} from '@/shared/config/routes';
import {LeaseInfo} from '@/entities/lease';
import type {components} from '@/shared/api/dto';
import type {PropertyPageStatus} from '../lib/get-property-page-status';
import { DetailSection } from '@/shared/ui/detail-section';
import styles from './PropertyLeaseCard.module.css';
import {SectionHeader} from '@/shared/ui/section-header';

type LeaseResponse = components['schemas']['LeaseResponse'];

export type PropertyLeaseCardProps = {
    readonly lease: LeaseResponse | undefined;
    readonly status: PropertyPageStatus;
    readonly propertyId: string;
    readonly onPayRent?: () => void;
    readonly onEndLease?: () => void;
    readonly isPayRentLoading?: boolean;
};

export function PropertyLeaseCard({
                                      lease,
                                      status,
                                      propertyId,
                                      onPayRent,
                                      onEndLease,
                                      isPayRentLoading = false,
                                  }: PropertyLeaseCardProps): JSX.Element {
    const leaseNewHref = `${ROUTES.leaseNew}?propertyId=${propertyId}`;
    const operationsHref = lease?.property_id
        ? `${ROUTES.financeOperations}?property_id=${lease.property_id}`
        : ROUTES.financeOperations;
    const showRentActions = status === 'rented';
    const showResolveActions = status === 'requires_action' && lease;
    const showCreateAction = status === 'free';
    const showUnavailableState =
        status !== 'awaiting_start' &&
        !showRentActions &&
        !showResolveActions &&
        !showCreateAction;
    const showActions =
        showRentActions ||
        showResolveActions ||
        showCreateAction ||
        showUnavailableState;

    return (
        <DetailSection>
            <SectionHeader title="Аренда" href={lease ? ROUTES.lease(lease.id) : ROUTES.propertyLeases(propertyId)}/>

            {lease ? (
                <NextLink href={ROUTES.lease(lease.id)} className={styles.card}>
                    <LeaseInfo lease={lease}/>
                </NextLink>
            ) : (
                <div className={styles.empty}>
                    <p className={styles.emptyText}>Объект свободен</p>
                </div>
            )}

            {showActions && (
                <div className={styles.actions}>
                    {showRentActions ? (
                        <>
                            {onPayRent ? (
                                <Button
                                    variant="primary"
                                    fullWidth
                                    onClick={onPayRent}
                                    loading={isPayRentLoading}
                                    type="button"
                                >
                                    Оплатить аренду
                                </Button>
                            ) : (
                                <LinkButton
                                    href={operationsHref}
                                    variant="primary"
                                    fullWidth
                                >
                                    Оплатить аренду
                                </LinkButton>
                            )}
                            <LinkButton
                                href={operationsHref}
                                variant="secondary"
                                fullWidth
                            >
                                Все операции
                            </LinkButton>
                        </>
                    ) : showResolveActions ? (
                        <>
                            <LinkButton href={ROUTES.lease(lease.id)} variant="primary" fullWidth>
                                Продлить текущую
                            </LinkButton>
                            <Button
                                variant="secondary"
                                fullWidth
                                onClick={onEndLease}
                                type="button"
                            >
                                Завершить аренду
                            </Button>
                        </>
                    ) : showCreateAction ? (
                        <>
                            <LinkButton href={leaseNewHref} variant="primary" fullWidth>
                                Создать аренду
                            </LinkButton>
                            <LinkButton href={ROUTES.propertyLeases(propertyId)} variant="secondary" fullWidth>
                                История аренд
                            </LinkButton>
                        </>
                    ) : (
                        <span className={styles.disabledText}>
              Аренда недоступна в этом статусе
            </span>
                    )}
                </div>
            )}
        </DetailSection>
    );
}
