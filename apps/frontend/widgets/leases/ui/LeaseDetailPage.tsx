'use client';

import {type JSX, useCallback, useMemo, useState,} from 'react';
import {useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import NextLink from 'next/link';
import {ROUTES} from '@/shared/config/routes';
import {useCompleteLease, useLease,} from '@/features/leases/api/hooks';
import {useOperationCategories} from '@/features/operation-categories/api';
import {useInfiniteOperations} from '@/features/operations/api/hooks';
import {useProperty} from '@/features/properties/api/hooks';
import {useSubscription} from '@/features/subscription/api/hooks';
import {isSubscriptionReadonly} from '@/features/subscription/lib/is-subscription-readonly';
import {Button} from '@/shared/ui/button';
import {ConfirmModal} from '@/shared/ui/confirm-modal';
import {PageHeader} from '@/shared/ui/page-header';
import {PropertyDetailSection} from '@/widgets/property-detail';
import {OperationListItem} from '@/widgets/operations/ui/OperationListItem';
import {StatusBadge} from '@/widgets/dashboard/ui/StatusBadge';
import {SectionHeader} from '@/widgets/dashboard/ui/SectionHeader';
import {FinanceLoading} from '@/widgets/finance/ui/FinanceLoading';
import {FinanceErrorState} from '@/widgets/finance/ui/FinanceErrorState';
import {LeaseDetailLoading} from './LeaseDetailLoading';
import {LeaseActionMenu} from './LeaseActionMenu';
import {SubscriptionReadonlyBanner} from '@/widgets/finance/ui/SubscriptionReadonlyBanner';
import {formatMoneyKopecks} from '@/shared/lib/format-money';
import {getTenantContactFullName} from '@/entities/tenant-contact/lib/get-tenant-contact-full-name';
import type {components} from '@/shared/api/generated';
import styles from './LeaseDetailPage.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type OperationResponse = components['schemas']['OperationResponse'];

function formatDateLabel(iso: string): string {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) {
        return iso;
    }
    return date.toLocaleDateString('ru-RU');
}

export type LeaseDetailPageProps = {
    readonly id: string;
};

function TenantCard({
                        tenantContact,
                    }: {
    readonly tenantContact: LeaseResponse['tenant_contact'];
}): JSX.Element {
    if (!tenantContact) {
        return (
            <PropertyDetailSection>
                <SectionHeader title="Арендатор" href={ROUTES.tenants}/>
                <div className={styles.card}>
                    <p className={styles.emptyText}>Арендатор не указан</p>
                </div>
            </PropertyDetailSection>
        );
    }

    const displayName = getTenantContactFullName(tenantContact) || tenantContact.name;

    return (
        <PropertyDetailSection>
            <SectionHeader title="Арендатор" href={ROUTES.tenant(tenantContact.id)}/>
            <NextLink
                href={ROUTES.tenant(tenantContact.id)}
                className={styles.card}
            >
                <p className={styles.detailValue}>{displayName}</p>
                {tenantContact.phone && (
                    <p className={styles.detailValue}>{tenantContact.phone}</p>
                )}
                {tenantContact.email && (
                    <p className={styles.detailValue}>{tenantContact.email}</p>
                )}
                {tenantContact.comment && (
                    <p className={styles.detailComment}>{tenantContact.comment}</p>
                )}
            </NextLink>
        </PropertyDetailSection>
    );
}

function TermsCard({lease}: { readonly lease: LeaseResponse }): JSX.Element {
    const endDate = lease.end_date ?? null;

    return (
        <PropertyDetailSection>
            <SectionHeader title="Условия аренды"/>
            <div className={styles.card}>
                <div className={styles.detailRow}>
                    <dt className={styles.detailLabel}>Начало аренды</dt>
                    <dd className={styles.detailValue}>
                        {formatDateLabel(lease.start_date)}
                    </dd>
                </div>
                <div className={styles.detailRow}>
                    <dt className={styles.detailLabel}>Конец аренды</dt>
                    <dd className={styles.detailValue}>
                        {endDate ? formatDateLabel(endDate) : 'Бессрочно'}
                    </dd>
                </div>
                <div className={styles.detailRow}>
                    <dt className={styles.detailLabel}>Арендная плата</dt>
                    <dd className={styles.detailValue}>
                        {formatMoneyKopecks(lease.rent_amount_kopecks)}
                    </dd>
                </div>
                <div className={styles.detailRow}>
                    <dt className={styles.detailLabel}>День оплаты</dt>
                    <dd className={styles.detailValue}>{lease.payment_day}-е число</dd>
                </div>
                <div className={styles.detailRow}>
                    <dt className={styles.detailLabel}>Залог</dt>
                    <dd className={styles.detailValue}>
                        {formatMoneyKopecks(lease.deposit_amount_kopecks)}
                    </dd>
                </div>
                {lease.comment && (
                    <div className={styles.detailRow}>
                        <dt className={styles.detailLabel}>Комментарий</dt>
                        <dd className={styles.detailValue}>{lease.comment}</dd>
                    </div>
                )}
            </div>
        </PropertyDetailSection>
    );
}

function OperationsSection({
                               operations,
                               isLoading,
                               isError,
                               isFetching,
                               hasNextPage,
                               isFetchNextPageError,
                               isFetchingNextPage,
                               onRetry,
                               onLoadMore,
                           }: {
    readonly operations: ReadonlyArray<OperationResponse>;
    readonly isLoading: boolean;
    readonly isError: boolean;
    readonly isFetching: boolean;
    readonly hasNextPage: boolean;
    readonly isFetchNextPageError: boolean;
    readonly isFetchingNextPage: boolean;
    readonly onRetry: () => void;
    readonly onLoadMore: () => void;
}): JSX.Element {
    if (isLoading) {
        return (
            <PropertyDetailSection>
                <SectionHeader title="Арендная плата"/>
                <div className={styles.card}>
                    <FinanceLoading/>
                </div>
            </PropertyDetailSection>
        );
    }

    if (isError) {
        return (
            <PropertyDetailSection>
                <SectionHeader title="Арендная плата"/>
                <div className={styles.card}>
                    <p className={styles.emptyText}>
                        Не удалось загрузить арендные операции.
                    </p>
                    <Button
                        type="button"
                        variant="secondary"
                        size="medium"
                        loading={isFetching}
                        onClick={onRetry}
                    >
                        Повторить
                    </Button>
                </div>
            </PropertyDetailSection>
        );
    }

    if (operations.length === 0) {
        return (
            <PropertyDetailSection>
                <SectionHeader title="Арендная плата"/>
                <div className={styles.card}>
                    <p className={styles.emptyText}>Арендных операций пока нет</p>
                </div>
            </PropertyDetailSection>
        );
    }

    return (
        <PropertyDetailSection>
            <SectionHeader title="Арендная плата"/>
            <ul className={styles.operationsList}>
                {operations.map((operation) => (
                    <li key={operation.id}>
                        <OperationListItem operation={operation}/>
                    </li>
                ))}
            </ul>
            {(hasNextPage || isFetchNextPageError) && (
                <div className={styles.loadMore}>
                    <Button
                        variant="secondary"
                        size="medium"
                        loading={isFetchingNextPage}
                        onClick={onLoadMore}
                    >
                        {isFetchNextPageError ? 'Повторить' : 'Показать ещё'}
                    </Button>
                    {isFetchNextPageError && (
                        <span className={styles.loadMoreError} role="alert">
                            Не удалось загрузить следующие операции
                        </span>
                    )}
                </div>
            )}
        </PropertyDetailSection>
    );
}

export function LeaseDetailPage({id}: LeaseDetailPageProps): JSX.Element {
    const router = useRouter();

    const leaseQuery = useLease(id);
    const categoriesQuery = useOperationCategories('income');
    const rentCategoryId = categoriesQuery.data?.find(
        (category) => category.code === 'rent',
    )?.id;
    const rentOperationsQuery = useInfiniteOperations(
        {
            lease_id: id,
            category_id: rentCategoryId ? [rentCategoryId] : undefined,
            sort: 'operation_date_asc',
        },
        {enabled: Boolean(rentCategoryId)},
    );
    const {data: subscription, isPending: isSubscriptionPending} = useSubscription();
    const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

    const completeLease = useCompleteLease();

    const [isCompleteModalOpen, setCompleteModalOpen] = useState(false);

    const lease = leaseQuery.data;
    const propertyQuery = useProperty(lease?.property_id ?? '');
    const propertyName = propertyQuery.data?.name ?? 'Объект';
    const rentOperations = useMemo(
        () => rentOperationsQuery.data?.pages.flatMap((page) => page.items) ?? [],
        [rentOperationsQuery.data],
    );

    const isLoading = leaseQuery.isPending;
    const isError = leaseQuery.isError;
    const isFetching = leaseQuery.isFetching;
    const canCompleteLease =
        lease !== undefined &&
        !readonly &&
        (lease.status === 'active' || lease.status === 'requires_action');
    const handleRetry = useCallback(() => {
        if (leaseQuery.isError) {
            leaseQuery.refetch();
        }
    }, [leaseQuery]);

    const handleOperationsRetry = useCallback(() => {
        if (!rentCategoryId) {
            categoriesQuery.refetch();
            return;
        }
        rentOperationsQuery.refetch();
    }, [rentCategoryId, categoriesQuery, rentOperationsQuery]);

    const handleLoadMoreOperations = useCallback(() => {
        void rentOperationsQuery.fetchNextPage();
    }, [rentOperationsQuery]);

    const confirmComplete = useCallback(() => {
        void notify.scenarios.leases.completed(completeLease.mutateAsync(id));
    }, [completeLease, id]);

    if (!id) {
        return (
            <FinanceErrorState
                onRetry={() => router.push(ROUTES.properties)}
                isLoading={false}
            />
        );
    }

    const title = lease ? (
        <NextLink
            href={ROUTES.property(lease.property_id)}
            className={styles.titleLink}
        >
            <span className={styles.title}>{propertyName}</span>
            <StatusBadge status={lease.status}/>
        </NextLink>
    ) : (
        <span className={styles.title}>Аренда</span>
    );

    return (
        <div className={styles.root}>
            <SubscriptionReadonlyBanner/>

            <PageHeader
                title={title}
                backHref={lease ? ROUTES.property(lease.property_id) : ROUTES.properties}
                actions={
                    <LeaseActionMenu
                        leaseId={id}
                        canEdit={!readonly && lease !== undefined && !isLoading}
                        canComplete={canCompleteLease}
                        onComplete={() => setCompleteModalOpen(true)}
                    />
                }
            />

            {isLoading && <LeaseDetailLoading/>}

            {!isLoading && isError && (
                <FinanceErrorState onRetry={handleRetry} isLoading={isFetching}/>
            )}

            {!isLoading && !isError && lease && (
                <>
                    <TenantCard tenantContact={lease.tenant_contact}/>
                    <TermsCard lease={lease}/>

                    <OperationsSection
                        operations={rentOperations}
                        isLoading={
                            rentOperationsQuery.isPending &&
                            !(!rentCategoryId && categoriesQuery.isError)
                        }
                        isError={
                            (rentOperationsQuery.isError && !rentOperationsQuery.data) ||
                            (!rentCategoryId && categoriesQuery.isError)
                        }
                        isFetching={
                            rentOperationsQuery.isFetching ||
                            categoriesQuery.isFetching
                        }
                        hasNextPage={rentOperationsQuery.hasNextPage}
                        isFetchNextPageError={rentOperationsQuery.isFetchNextPageError}
                        isFetchingNextPage={rentOperationsQuery.isFetchingNextPage}
                        onRetry={handleOperationsRetry}
                        onLoadMore={handleLoadMoreOperations}
                    />
                </>
            )}

            <ConfirmModal
                isOpen={isCompleteModalOpen}
                title="Завершить аренду?"
                description="Информацию по этой аренде можно будет посмотреть в разделе «Аренда»."
                confirmLabel="Завершить аренду"
                onClose={() => setCompleteModalOpen(false)}
                onConfirm={confirmComplete}
            />
        </div>
    );
}
