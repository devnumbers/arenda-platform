'use client';

import {type JSX, useCallback, useMemo, useState,} from 'react';
import {useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/toast';
import NextLink from 'next/link';
import {ROUTES} from '@/shared/config/routes';
import {useCompleteLease, useLease, useReturnDeposit,} from '@/features/leases/api/hooks';
import {useOperations} from '@/features/operations/api/hooks';
import {useProperty} from '@/features/properties/api/hooks';
import {useSubscription} from '@/features/subscription/api/hooks';
import {isSubscriptionReadonly} from '@/features/subscription/lib/is-subscription-readonly';
import {ApiError} from '@/shared/api/errors';
import {Button} from '@/shared/ui/button';
import {ConfirmModal} from '@/shared/ui/confirm-modal';
import {PageHeader} from '@/shared/ui/page-header';
import {PropertyDetailSection} from '@/widgets/property-detail';
import {OperationListItem} from '@/widgets/operations/ui/OperationListItem';
import {StatusBadge} from '@/widgets/dashboard/ui/StatusBadge';
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
                <h2 className={styles.sectionTitle}>Арендатор</h2>
                <div className={styles.card}>
                    <p className={styles.emptyText}>Арендатор не указан</p>
                </div>
            </PropertyDetailSection>
        );
    }

    const displayName = getTenantContactFullName(tenantContact) || tenantContact.name;

    return (
        <PropertyDetailSection>
            <h2 className={styles.sectionTitle}>Арендатор</h2>
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
            <h2 className={styles.sectionTitle}>Условия аренды</h2>
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
                               onRetry,
                           }: {
    readonly operations: ReadonlyArray<OperationResponse>;
    readonly isLoading: boolean;
    readonly isError: boolean;
    readonly isFetching: boolean;
    readonly onRetry: () => void;
}): JSX.Element {
    if (isLoading) {
        return (
            <PropertyDetailSection>
                <h2 className={styles.sectionTitle}>Арендная плата</h2>
                <div className={styles.card}>
                    <FinanceLoading/>
                </div>
            </PropertyDetailSection>
        );
    }

    if (isError) {
        return (
            <PropertyDetailSection>
                <h2 className={styles.sectionTitle}>Арендная плата</h2>
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
                <h2 className={styles.sectionTitle}>Арендная плата</h2>
                <div className={styles.card}>
                    <p className={styles.emptyText}>Арендных операций пока нет</p>
                </div>
            </PropertyDetailSection>
        );
    }

    return (
        <PropertyDetailSection>
            <h2 className={styles.sectionTitle}>Арендная плата</h2>
            <ul className={styles.operationsList}>
                {operations.map((operation) => (
                    <li key={operation.id}>
                        <OperationListItem operation={operation}/>
                    </li>
                ))}
            </ul>
        </PropertyDetailSection>
    );
}

export function LeaseDetailPage({id}: LeaseDetailPageProps): JSX.Element {
    const router = useRouter();

    const leaseQuery = useLease(id);
    const rentOperationsQuery = useOperations({lease_id: id, category: 'rent', sort: 'operation_date_asc'});
    const depositReturnQuery = useOperations({
        lease_id: id,
        category: 'deposit_return',
        limit: 1,
    });
    const {data: subscription, isPending: isSubscriptionPending} = useSubscription();
    const readonly = isSubscriptionPending || isSubscriptionReadonly(subscription);

    const completeLease = useCompleteLease();
    const returnDeposit = useReturnDeposit();

    const [isCompleteModalOpen, setCompleteModalOpen] = useState(false);
    const [isDepositModalOpen, setDepositModalOpen] = useState(false);

    const lease = leaseQuery.data;
    const propertyQuery = useProperty(lease?.property_id ?? '');
    const propertyName = propertyQuery.data?.name ?? 'Объект';
    const rentOperations = useMemo(
        () => rentOperationsQuery.data?.items ?? [],
        [rentOperationsQuery.data],
    );
    const hasDepositReturn = useMemo(
        () => (depositReturnQuery.data?.items ?? []).length > 0,
        [depositReturnQuery.data],
    );

    const isLoading = leaseQuery.isPending;
    const isError = leaseQuery.isError;
    const isFetching = leaseQuery.isFetching;
    const canCompleteLease =
        lease !== undefined &&
        !readonly &&
        (lease.status === 'active' || lease.status === 'requires_action');
    const canReturnDeposit =
        lease !== undefined &&
        !readonly &&
        !depositReturnQuery.isPending &&
        !depositReturnQuery.isError &&
        (lease.status === 'completed' || lease.status === 'requires_action') &&
        lease.deposit_amount_kopecks > 0 &&
        !hasDepositReturn;
    const handleRetry = useCallback(() => {
        if (leaseQuery.isError) {
            leaseQuery.refetch();
        }
    }, [leaseQuery]);

    const handleOperationsRetry = useCallback(() => {
        rentOperationsQuery.refetch();
    }, [rentOperationsQuery]);

    const confirmComplete = useCallback(() => {
        void notify.promise(completeLease.mutateAsync(id), {
            loading: 'Завершаем аренду...',
            success: 'Аренда завершена',
            error: (error) =>
                (error as ApiError).detail ?? 'Не удалось завершить аренду',
        });
    }, [completeLease, id]);

    const confirmReturnDeposit = useCallback(() => {
        void notify.promise(returnDeposit.mutateAsync(id), {
            loading: 'Возвращаем залог...',
            success: 'Залог возвращён',
            error: (error) =>
                (error as ApiError).detail ?? 'Не удалось вернуть залог',
        });
    }, [returnDeposit, id]);

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
                        canReturnDeposit={canReturnDeposit}
                        onComplete={() => setCompleteModalOpen(true)}
                        onReturnDeposit={() => setDepositModalOpen(true)}
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
                        isLoading={rentOperationsQuery.isPending}
                        isError={rentOperationsQuery.isError}
                        isFetching={rentOperationsQuery.isFetching}
                        onRetry={handleOperationsRetry}
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
            <ConfirmModal
                isOpen={isDepositModalOpen}
                title="Вернуть залог?"
                confirmLabel="Вернуть залог"
                onClose={() => setDepositModalOpen(false)}
                onConfirm={confirmReturnDeposit}
            />
        </div>
    );
}
