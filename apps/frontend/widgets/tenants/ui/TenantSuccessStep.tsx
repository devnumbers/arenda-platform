'use client';

import type {JSX} from 'react';
import {Button} from '@/shared/ui/button';
import {Icon} from '@/shared/ui/icon';
import {BoldProfile, BoldWallet} from '@/shared/assets/icons';
import styles from './TenantSuccessStep.module.css';

export type TenantSuccessStepProps = {
    readonly onAddLater: () => void;
    readonly onAddOperations: () => void;
};

export function TenantSuccessStep({
                                      onAddLater,
                                      onAddOperations,
                                  }: TenantSuccessStepProps): JSX.Element {
    return (
        <div className={styles.root}>
            <div className={styles.card}>
                <div className={styles.iconWrapper}>
                    <BoldProfile className={styles.illustration}/>
                </div>

                <div className={styles.text}>
                    <h2 className={styles.heading}>Арендатор добавлен</h2>
                </div>
            </div>

            <div className={styles.actions}>
                <Button
                    type="button"
                    variant="secondary"
                    size="large"
                    fullWidth
                    onClick={onAddLater}
                >
                    Добавить позже
                </Button>
                <Button
                    type="button"
                    variant="primary"
                    size="large"
                    fullWidth
                    leftIcon={
                        <Icon size="l">
                            <BoldWallet/>
                        </Icon>
                    }
                    onClick={onAddOperations}
                >
                    Добавить операции
                </Button>
            </div>
        </div>
    );
}
