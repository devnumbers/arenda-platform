'use client';

import type {JSX} from 'react';

import {parseDate} from '@internationalized/date';
import {Button} from '@/shared/ui/button';
import {DateSelect} from '@/shared/ui/date-select';
import {LinkButton} from '@/shared/ui/link-button';
import {Select} from '@/shared/ui/select';
import type {TenantContact} from '@/entities/tenant-contact';
import {getTenantContactFullName} from '@/entities/tenant-contact';
import {PaymentDayPicker} from './PaymentDayPicker';
import styles from './LeaseDatesStep.module.css';

export type LeaseDatesStepProps = {
    readonly tenantContactId: string;
    readonly onTenantChange: (id: string) => void;
    readonly tenantContacts?: TenantContact[];
    readonly isTenantListEmpty?: boolean;
    readonly isTenantContactsLoading?: boolean;
    readonly createTenantHref: string;
    readonly paymentDay?: number;
    readonly startDate?: string;
    readonly endDate?: string;
    readonly onPaymentDayChange: (day: number) => void;
    readonly onStartDateChange: (date: string) => void;
    readonly onEndDateChange: (date: string) => void;
    readonly onSubmit: () => void;
    readonly isLoading: boolean;
};

export function LeaseDatesStep({
                                   tenantContactId,
                                   onTenantChange,
                                   tenantContacts,
                                   isTenantListEmpty,
                                   isTenantContactsLoading,
                                   createTenantHref,
                                   paymentDay,
                                   startDate,
                                   endDate,
                                   onPaymentDayChange,
                                   onStartDateChange,
                                   onEndDateChange,
                                   onSubmit,
                                   isLoading,
                               }: LeaseDatesStepProps): JSX.Element {
    const handleStartDateChange = (date: string | undefined) => {
        if (!date) return;
        onStartDateChange(date);
    };

    const handleEndDateChange = (date: string | undefined) => {
        onEndDateChange(date ?? '');
    };

    const startDateValid = startDate !== undefined && startDate !== '';
    const datesOrdered =
        !startDateValid ||
        endDate === undefined ||
        endDate === '' ||
        parseDate(startDate).compare(parseDate(endDate)) <= 0;

    const isValid =
        paymentDay !== undefined &&
        paymentDay >= 1 &&
        paymentDay <= 31 &&
        startDateValid &&
        datesOrdered;

    return (
        <div className={styles.root}>
            <h2 className={styles.heading}>Даты аренды</h2>
            <div className={styles.fields}>
                <Select
                    label="Арендатор"
                    placeholder="Не указан"
                    value={tenantContactId}
                    onChange={onTenantChange}
                    disabled={isTenantListEmpty}
                    loading={isTenantContactsLoading}
                    options={[
                        {value: '', label: 'Не указан'},
                        ...(tenantContacts?.map((contact) => ({
                            value: contact.id,
                            label: getTenantContactFullName(contact) || contact.name,
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
                <PaymentDayPicker value={paymentDay} onChange={onPaymentDayChange}/>
                <div className={styles.dateRow}>
                    <DateSelect
                        label="Начало аренды"
                        value={startDate}
                        placeholder="Выбрать дату"
                        onChange={handleStartDateChange}
                        required
                    />
                    <DateSelect
                        label="Конец аренды"
                        value={endDate}
                        placeholder="Выбрать дату"
                        minValue={startDate}
                        onChange={handleEndDateChange}
                    />
                </div>
            </div>
            <div className={styles.submit}>
                <Button
                    type="button"
                    variant="primary"
                    size="large"
                    fullWidth
                    disabled={!isValid}
                    loading={isLoading}
                    onClick={onSubmit}
                >
                    Создать аренду
                </Button>
            </div>
        </div>
    );
}
