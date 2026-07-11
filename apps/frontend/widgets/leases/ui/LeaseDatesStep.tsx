'use client';

import type {JSX} from 'react';

import {parseDate} from '@internationalized/date';
import {Button} from '@/shared/ui/button';
import {DateSelect} from '@/shared/ui/date-select';
import {PaymentDayPicker} from './PaymentDayPicker';
import styles from './LeaseDatesStep.module.css';

export type LeaseDatesStepProps = {
    readonly paymentDay?: number;
    readonly startDate?: string;
    readonly endDate?: string;
    readonly onPaymentDayChange: (day: number) => void;
    readonly onStartDateChange: (date: string) => void;
    readonly onEndDateChange: (date: string) => void;
    readonly onSubmit: () => void;
    readonly isLoading: boolean;
    readonly error?: string;
};

export function LeaseDatesStep({
                                   paymentDay,
                                   startDate,
                                   endDate,
                                   onPaymentDayChange,
                                   onStartDateChange,
                                   onEndDateChange,
                                   onSubmit,
                                   isLoading,
                                   error,
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
                {error && <p className={styles.error}>{error}</p>}
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
