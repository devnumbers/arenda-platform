'use client';

import type {JSX} from 'react';
import {useMemo} from 'react';
import NextLink from 'next/link';
import {useParams} from 'next/navigation';
import type {components} from '@/shared/api/dto';
import {Plus} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {Icon} from '@/shared/ui/icon';
import {PageHeader} from '@/shared/ui/page-header';
import {LinkButton} from '@/shared/ui/link-button';
import {useProperty} from '@/features/properties';
import {usePropertyLeases} from '@/features/leases';
import {FinanceLoading} from '@/shared/ui/finance-loading';
import {FinanceErrorState} from '@/shared/ui/finance-error-state';
import styles from './PropertyLeasesPage.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeaseStatus = LeaseResponse['status'];

const OPEN_STATUSES = new Set<LeaseStatus>([
    'awaiting_start',
    'active',
    'requires_action',
]);

const STATUS_LABELS: Record<LeaseStatus, string> = {
    awaiting_start: 'Скоро начнётся',
    active: 'Активна',
    requires_action: 'Требует действия',
    completed: 'Завершена',
    archived: 'В архиве',
};

function isOpenLease(status: LeaseStatus): boolean {
    return OPEN_STATUSES.has(status);
}

function sortLeases(leases: ReadonlyArray<LeaseResponse>): LeaseResponse[] {
    return [...leases].sort((a, b) => {
        const aOpen = isOpenLease(a.status);
        const bOpen = isOpenLease(b.status);

        if (aOpen !== bOpen) {
            return aOpen ? -1 : 1;
        }

        return b.start_date.localeCompare(a.start_date);
    });
}

function formatDateLabel(iso: string): string {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
    if (!match) {
        return iso;
    }

    const [, year, month, day] = match;
    return `${day}.${month}.${year}`;
}

function formatPeriod(lease: LeaseResponse): string {
    const start = formatDateLabel(lease.start_date);
    const end = lease.end_date ? formatDateLabel(lease.end_date) : 'бессрочно';
    return `${start} - ${end}`;
}

function formatTenantName(lease: LeaseResponse): string {
    const tenant = lease.tenant_contact;
    if (!tenant) {
        return 'Арендатор не указан';
    }

    return [tenant.surname, tenant.name, tenant.patronymic]
        .filter(Boolean)
        .join(' ') || tenant.name;
}

function LeaseHistoryCard({lease}: { readonly lease: LeaseResponse }): JSX.Element {
    return (
        <NextLink href={ROUTES.lease(lease.id)} className={styles.card}>
            <div className={styles.cardHeader}>
                <div className={styles.cardTitleGroup}>
                    <span className={styles.tenant}>{formatTenantName(lease)}</span>
                    <span className={styles.period}>{formatPeriod(lease)}</span>
                </div>
                <span className={styles.status}>{STATUS_LABELS[lease.status]}</span>
            </div>

            <dl className={styles.details}>
                <div className={styles.detailItem}>
                    <dt>Арендная плата</dt>
                    <dd>{formatMoneyKopecks(lease.rent_amount_kopecks)}</dd>
                </div>
                <div className={styles.detailItem}>
                    <dt>День оплаты</dt>
                    <dd>{lease.payment_day}-е число</dd>
                </div>
                <div className={styles.detailItem}>
                    <dt>Залог</dt>
                    <dd>{formatMoneyKopecks(lease.deposit_amount_kopecks)}</dd>
                </div>
            </dl>
        </NextLink>
    );
}

export function PropertyLeasesPage(): JSX.Element {
    const params = useParams<{ id: string }>();
    const id = params.id ?? '';

    const propertyQuery = useProperty(id);
    const leasesQuery = usePropertyLeases(id);
    const leases = useMemo(
        () => sortLeases(leasesQuery.data?.items ?? []),
        [leasesQuery.data],
    );

    const isLoading = propertyQuery.isLoading || leasesQuery.isLoading;
    const isError = propertyQuery.isError || leasesQuery.isError;
    const isFetching = propertyQuery.isFetching || leasesQuery.isFetching;
    const propertyName = propertyQuery.data?.name ?? 'Мой объект';
    const isArchived = propertyQuery.data?.status === 'archived';
    const hasOpenLease = leases.some((lease) => isOpenLease(lease.status));
    const canCreateLease = !isLoading && !isError && !hasOpenLease && !isArchived;
    const showCreateLeaseAction = !isLoading && !isError && !hasOpenLease;

    const handleRetry = (): void => {
        propertyQuery.refetch();
        leasesQuery.refetch();
    };

    const title = (
        <div className={styles.heading}>
            <span className={styles.eyebrow}>{propertyName}</span>
            <span className={styles.title}>Все аренды</span>
        </div>
    );

    return (
        <div className={styles.root}>
            <PageHeader
                title={title}
                backHref={ROUTES.property(id)}
                actions={
                    isLoading ? (
                        <button
                            type="button"
                            disabled
                            className={styles.addButton}
                            aria-label="Создать аренду"
                        >
                            <Icon size="l">
                                <Plus/>
                            </Icon>
                        </button>
                    ) : isArchived && !isError && !hasOpenLease ? (
                        <button
                            type="button"
                            disabled
                            className={styles.addButton}
                            aria-label="Создать аренду"
                            title="Объект в архиве"
                        >
                            <Icon size="l">
                                <Plus/>
                            </Icon>
                        </button>
                    ) : canCreateLease ? (
                        <NextLink
                            href={`${ROUTES.leaseNew}?propertyId=${id}`}
                            className={styles.addButton}
                            aria-label="Создать аренду"
                        >
                            <Icon size="l">
                                <Plus/>
                            </Icon>
                        </NextLink>
                    ) : null
                }
            />

            {isLoading && <FinanceLoading/>}

            {!isLoading && isError && (
                <FinanceErrorState onRetry={handleRetry} isLoading={isFetching}/>
            )}

            {!isLoading && !isError && leases.length === 0 && (
                <section className={styles.empty}>
                    <h2 className={styles.emptyTitle}>Аренд пока нет</h2>
                    {showCreateLeaseAction && (
                        <LinkButton
                            href={`${ROUTES.leaseNew}?propertyId=${id}`}
                            variant="primary"
                            className={styles.btn}
                            size="medium"
                            disabled={isArchived}
                            title={isArchived ? 'Объект в архиве' : undefined}
                        >
                            Создать аренду
                        </LinkButton>
                    )}
                </section>
            )}

            {!isLoading && !isError && leases.length > 0 && (
                <ul className={styles.list}>
                    {leases.map((lease) => (
                        <li key={lease.id}>
                            <LeaseHistoryCard lease={lease}/>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}
