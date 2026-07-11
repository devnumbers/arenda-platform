'use client';

import type {JSX} from 'react';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import styles from './LeasePriceStep.module.css';

export type LeasePriceStepProps = {
    readonly rentAmount: string;
    readonly depositAmount: string;
    readonly onRentChange: (value: string) => void;
    readonly onDepositChange: (value: string) => void;
    readonly onNext: () => void;
};

export function LeasePriceStep({
                                   rentAmount,
                                   depositAmount,
                                   onRentChange,
                                   onDepositChange,
                                   onNext,
                               }: LeasePriceStepProps): JSX.Element {
    const isValid =
        rentAmount.trim() !== '' &&
        Number(rentAmount) > 0 &&
        (depositAmount.trim() === '' || Number(depositAmount) >= 0);

    return (
        <div className={styles.root}>
            <h2 className={styles.heading}>Цена и залог</h2>
            <div className={styles.fields}>
                <TextField
                    label="Арендная плата"
                    placeholder="Цена, ₽ в месяц"
                    type="number"
                    min={0}
                    required
                    value={rentAmount}
                    onChange={(e) => onRentChange(e.currentTarget.value)}
                    fullWidth
                />
                <TextField
                    label="Залог"
                    placeholder=" "
                    type="number"
                    min={0}
                    value={depositAmount}
                    onChange={(e) => onDepositChange(e.currentTarget.value)}
                    fullWidth
                />
            </div>
            <div className={styles.footer}>
                <Button
                    type="button"
                    variant="primary"
                    size="large"
                    fullWidth
                    disabled={!isValid}
                    onClick={onNext}
                >
                    Продолжить
                </Button>
            </div>
        </div>
    );
}
