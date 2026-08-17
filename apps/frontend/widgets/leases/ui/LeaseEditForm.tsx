'use client';

import {type ChangeEvent, type FormEvent, type JSX, useEffect, useMemo, useRef, useState,} from 'react';
import {useRouter} from 'next/navigation';
import {notify} from '@/shared/lib/notifications';
import {ROUTES} from '@/shared/config/routes';
import {goBack, RETURN_TO_PARAM} from '@/shared/lib/navigation';
import {useLease, useUpdateLease} from '@/features/leases/api/hooks';
import {useTenantContacts} from '@/features/tenant-contacts/api/hooks';
import {TextField} from '@/shared/ui/text-field';
import {Select} from '@/shared/ui/select';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
import {PageHeader} from '@/shared/ui/page-header';
import {IconButton} from '@/shared/ui/icon-button';
import {Cancel} from '@/shared/assets/icons';
import { DetailSection } from '@/shared/ui/detail-section';
import type {components} from '@/shared/api/dto';
import type {TenantContact} from '@/entities/tenant-contact/model/types';
import {getTenantContactFullName} from '@/entities/tenant-contact/lib/get-tenant-contact-full-name';
import {DateSelect} from '@/shared/ui/date-select';
import {useLeaseEditDraft} from '../lib/use-lease-edit-draft';
import {PaymentDayPicker} from './PaymentDayPicker';
import {LeaseEditFormLoading} from './LeaseEditFormLoading';
import styles from './LeaseEditForm.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeaseUpdateRequest = components['schemas']['LeaseUpdateRequest'];

type FormData = {
    tenantContactId: string;
    startDate: string;
    endDate: string;
    rentAmount: string;
    depositAmount: string;
    paymentDay: string;
    comment: string;
};

type FormErrors = {
    rentAmount?: string;
    depositAmount?: string;
    paymentDay?: string;
    startDate?: string;
    endDate?: string;
};

function kopecksToRubles(kopecks: number): string {
    return (kopecks / 100).toFixed(2);
}

function parseRublesToKopecks(value: string): number | undefined {
    const normalized = value.trim().replace(',', '.');
    if (normalized === '') {
        return undefined;
    }
    const number = Number(normalized);
    if (Number.isNaN(number) || number < 0) {
        return undefined;
    }
    return Math.round(number * 100);
}

function getTenantContactOptionLabel(contact: TenantContact): string {
    return getTenantContactFullName(contact) || contact.name;
}

function initializeForm(lease: LeaseResponse): FormData {
    return {
        tenantContactId: lease.tenant_contact?.id ?? '',
        startDate: lease.start_date,
        endDate: lease.end_date ?? '',
        rentAmount: kopecksToRubles(lease.rent_amount_kopecks),
        depositAmount: kopecksToRubles(lease.deposit_amount_kopecks),
        paymentDay: String(lease.payment_day),
        comment: lease.comment ?? '',
    };
}

type LeaseEditFormErrorProps = {
    readonly onRetry: () => void;
    readonly isLoading?: boolean;
};

function LeaseEditFormError({onRetry, isLoading = false}: LeaseEditFormErrorProps): JSX.Element {
    return (
        <div className={styles.errorCard} role="alert" aria-live="polite">
            <h2 className={styles.errorTitle}>Не удалось загрузить аренду</h2>
            <p className={styles.errorMessage}>
                Проверьте соединение и попробуйте снова
            </p>
            <Button
                variant="primary"
                size="medium"
                loading={isLoading}
                onClick={onRetry}
                type="button"
            >
                Повторить
            </Button>
        </div>
    );
}

export type LeaseEditFormProps = {
    readonly leaseId: string;
    readonly returnTo?: string;
    readonly preselectedTenantContactId?: string;
};

export function LeaseEditForm({leaseId, returnTo, preselectedTenantContactId}: LeaseEditFormProps): JSX.Element {
    const router = useRouter();

    const leaseQuery = useLease(leaseId);
    const updateLease = useUpdateLease();
    const {data: tenantContacts, isLoading: isTenantContactsLoading} = useTenantContacts();
    const {draft, isLoaded: isDraftLoaded, saveDraft, clearDraft} = useLeaseEditDraft(leaseId);

    const [form, setForm] = useState<FormData>({
        tenantContactId: '',
        startDate: '',
        endDate: '',
        rentAmount: '',
        depositAmount: '',
        paymentDay: '',
        comment: '',
    });
    const [errors, setErrors] = useState<FormErrors>({});
    const hasInitialized = useRef(false);

    useEffect(() => {
        const lease = leaseQuery.data;
        if (!lease || !isDraftLoaded || hasInitialized.current) return;

        // Initialize form state from loaded lease data once to avoid wiping
        // user edits on background refetch. A persisted draft wins over lease
        // data; an explicit tenant preselection from the URL wins over both.
        setForm({
            ...initializeForm(lease),
            ...draft,
            ...(preselectedTenantContactId ? {tenantContactId: preselectedTenantContactId} : {}),
        });
        hasInitialized.current = true;
    }, [leaseQuery.data, draft, isDraftLoaded, preselectedTenantContactId]);

    useEffect(() => {
        if (!hasInitialized.current) return;

        // Persist every form change so returning from the tenant-create flow
        // restores the edits made before leaving the page.
        saveDraft(form);
    }, [form, saveDraft]);

    const handleTenantChange = (tenantContactId: string) => {
        setForm((prev) => ({...prev, tenantContactId}));
    };

    const updateField = (field: keyof FormData, value: string) => {
        setForm((prev) => ({...prev, [field]: value}));
        setErrors((prev) => {
            const next = {...prev};
            if (field in next) {
                delete (next as Record<keyof FormData, string | undefined>)[field];
            }
            return next;
        });
    };

    const handleTextChange =
        (field: keyof FormData) =>
            (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
                updateField(field, event.currentTarget.value);
            };

    const handleStartDateChange = (date: string | undefined) => {
        updateField('startDate', date ?? '');
    };

    const handleEndDateChange = (date: string | undefined) => {
        updateField('endDate', date ?? '');
    };

    const handlePaymentDayChange = (day: number) => {
        updateField('paymentDay', String(day));
    };

    const getFormErrors = (): FormErrors => {
        const next: FormErrors = {};

        const rentKopecks = parseRublesToKopecks(form.rentAmount);
        if (rentKopecks === undefined) {
            next.rentAmount = 'Введите сумму аренды';
        }

        if (
            form.depositAmount.trim() !== '' &&
            parseRublesToKopecks(form.depositAmount) === undefined
        ) {
            next.depositAmount = 'Введите корректную сумму залога';
        }

        const paymentDay = Number(form.paymentDay);
        if (Number.isNaN(paymentDay) || paymentDay < 1 || paymentDay > 31) {
            next.paymentDay = 'Введите число от 1 до 31';
        }

        if (!form.startDate) {
            next.startDate = 'Укажите дату начала';
        }

        if (
            form.endDate &&
            form.startDate &&
            new Date(form.endDate) < new Date(form.startDate)
        ) {
            next.endDate = 'Дата окончания не может быть раньше начала';
        }

        return next;
    };

    const isFormValid = (): boolean => Object.keys(getFormErrors()).length === 0;

    const validate = (): boolean => {
        const next = getFormErrors();
        setErrors(next);
        return Object.keys(next).length === 0;
    };

    const hasChanges = useMemo(() => {
        const lease = leaseQuery.data;
        if (!lease) {
            return false;
        }

        return (
            form.tenantContactId !== (lease.tenant_contact?.id ?? '') ||
            form.startDate !== lease.start_date ||
            form.endDate !== (lease.end_date ?? '') ||
            parseRublesToKopecks(form.rentAmount) !== lease.rent_amount_kopecks ||
            parseRublesToKopecks(form.depositAmount) !== lease.deposit_amount_kopecks ||
            form.paymentDay !== String(lease.payment_day) ||
            form.comment.trim() !== (lease.comment ?? '').trim()
        );
    }, [
        form.tenantContactId,
        form.startDate,
        form.endDate,
        form.rentAmount,
        form.depositAmount,
        form.paymentDay,
        form.comment,
        leaseQuery.data,
    ]);

    const canSubmit = isFormValid() && !updateLease.isPending && hasChanges;

    const isTenantListEmpty = !isTenantContactsLoading && tenantContacts?.length === 0;

    const currentUrl = returnTo
        ? `${ROUTES.leaseEdit(leaseId)}?${RETURN_TO_PARAM}=${encodeURIComponent(returnTo)}`
        : ROUTES.leaseEdit(leaseId);
    const createTenantHref = `${ROUTES.tenantNew}?${RETURN_TO_PARAM}=${encodeURIComponent(currentUrl)}`;

    const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();

        if (!validate()) {
            return;
        }

        const rentKopecks = parseRublesToKopecks(form.rentAmount);
        const depositKopecks = parseRublesToKopecks(form.depositAmount);
        const paymentDay = Number(form.paymentDay);

        if (rentKopecks === undefined || Number.isNaN(paymentDay)) {
            return;
        }

        const data: LeaseUpdateRequest = {
            tenant_contact_id: form.tenantContactId || undefined,
            clear_tenant_contact: !form.tenantContactId,
            start_date: form.startDate,
            ...(form.endDate ? {end_date: form.endDate} : {}),
            rent_amount_kopecks: rentKopecks,
            deposit_amount_kopecks: depositKopecks ?? 0,
            payment_day: paymentDay,
            ...(form.comment.trim() ? {comment: form.comment.trim()} : {}),
        };

        try {
            await updateLease.mutateAsync({id: leaseId, data});
            notify.scenarios.leases.updated();
            clearDraft();
            if (returnTo) {
                router.replace(returnTo);
            } else {
                goBack(router, ROUTES.lease(leaseId));
            }
        } catch (error: unknown) {
            notify.scenarios.leases.leaseSaveError(error);
        }
    };

    return (
        <div className={styles.root}>
            <PageHeader
                title="Редактирование аренды"
                backHref={returnTo ?? ROUTES.lease(leaseId)}
                actions={
                    <IconButton
                        variant="secondary"
                        size="large"
                        icon={<Cancel />}
                        aria-label="Отменить"
                        onClick={() => {
                            clearDraft();
                            goBack(router, returnTo ?? ROUTES.lease(leaseId));
                        }}
                    />
                }
            />

            {leaseQuery.isPending && <LeaseEditFormLoading />}

            {!leaseQuery.isPending && (leaseQuery.isError || !leaseQuery.data) && (
                <LeaseEditFormError
                    onRetry={leaseQuery.refetch}
                    isLoading={leaseQuery.isFetching}
                />
            )}

            {!leaseQuery.isPending && leaseQuery.data && (
                <form className={styles.form} onSubmit={handleSubmit}>
                    <DetailSection>
                        <h2 className={styles.sectionTitle}>Условия аренды</h2>
                        <div className={styles.fields}>
                            <Select
                                label="Арендатор"
                                placeholder="Не указан"
                                value={form.tenantContactId}
                                onChange={handleTenantChange}
                                disabled={isTenantListEmpty}
                                loading={isTenantContactsLoading}
                                options={[
                                    {value: '', label: 'Не указан'},
                                    ...(tenantContacts?.map((contact) => ({
                                        value: contact.id,
                                        label: getTenantContactOptionLabel(contact),
                                    })) ?? []),
                                ]}
                            />
                            {isTenantListEmpty && (
                                <LinkButton
                                    href={createTenantHref}
                                    variant="secondary"
                                    size="medium"
                                    className={styles.emptyLink}
                                >
                                    Добавить арендатора
                                </LinkButton>
                            )}

                            <PaymentDayPicker
                                value={form.paymentDay ? Number(form.paymentDay) : undefined}
                                onChange={handlePaymentDayChange}
                                error={errors.paymentDay}
                            />

                            <div className={styles.dateRow}>
                                <DateSelect
                                    label="Начало аренды"
                                    value={form.startDate || undefined}
                                    placeholder="Выбрать дату"
                                    onChange={handleStartDateChange}
                                    required
                                    error={errors.startDate}
                                />
                                <DateSelect
                                    label="Конец аренды"
                                    value={form.endDate || undefined}
                                    placeholder="Выбрать дату"
                                    minValue={form.startDate}
                                    onChange={handleEndDateChange}
                                    error={errors.endDate}
                                />
                            </div>

                            <TextField
                                label="Арендная плата, ₽"
                                type="number"
                                min={0}
                                step="0.01"
                                required
                                fullWidth
                                value={form.rentAmount}
                                onChange={handleTextChange('rentAmount')}
                                error={errors.rentAmount}
                            />
                            <TextField
                                label="Залог, ₽"
                                type="number"
                                min={0}
                                step="0.01"
                                fullWidth
                                value={form.depositAmount}
                                onChange={handleTextChange('depositAmount')}
                                error={errors.depositAmount}
                            />
                            <TextField
                                label="Комментарий"
                                placeholder="Дополнительная информация"
                                multiline
                                maxLength={500}
                                showCounter
                                fullWidth
                                value={form.comment}
                                onChange={handleTextChange('comment')}
                            />
                        </div>
                    </DetailSection>

                    <div className={styles.actions}>
                        <Button
                            type="submit"
                            variant="primary"
                            size="large"
                            fullWidth
                            loading={updateLease.isPending}
                            disabled={!canSubmit}
                        >
                            Сохранить
                        </Button>
                    </div>
                </form>
            )}
        </div>
    );
}
