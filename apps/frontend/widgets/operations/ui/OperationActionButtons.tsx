'use client';

import type {JSX} from 'react';
import type {components} from '@/shared/api/generated';
import type {OperationStatus} from '@/entities/operation/model/types';
import {Button} from '@/shared/ui/button';
import styles from './OperationActionButtons.module.css';

type OperationType = components['schemas']['OperationType'];

export type OperationActionButtonsProps = {
    readonly type?: OperationType;
    readonly status?: OperationStatus;
    readonly disabled?: boolean;
    readonly loadingAction?: 'complete' | 'markIncomplete' | 'delete' | null;
    readonly onComplete: () => void;
    readonly onMarkIncomplete: () => void;
    readonly onEdit: () => void;
    readonly onDelete: () => void;
};

export function OperationActionButtons({
                                           type,
                                           status,
                                           disabled,
                                           loadingAction,
                                           onComplete,
                                           onMarkIncomplete,
                                           onEdit,
                                           onDelete,
                                       }: OperationActionButtonsProps): JSX.Element {
    const isIncome = type === 'income';

    const canComplete = status === 'pending' || status === 'overdue' || status === 'unconfirmed';
    const canMarkIncomplete = status === 'paid' || status === 'received';

    const completeLabel =
        status === 'unconfirmed'
            ? 'Подтвердить'
            : isIncome
                ? 'Отметить полученной'
                : 'Отметить оплаченной';
    const markIncompleteLabel = isIncome ? 'Отметить не полученной' : 'Отметить не оплаченной';

    return (
        <div className={styles.actions}>
            {canComplete && (
                <Button
                    variant="primary"
                    size="large"
                    className={styles.actionButton}
                    loading={loadingAction === 'complete'}
                    disabled={disabled}
                    onClick={onComplete}
                >
                    {completeLabel}
                </Button>
            )}
            {canMarkIncomplete && (
                <Button
                    variant="primary"
                    size="large"
                    className={styles.actionButton}
                    loading={loadingAction === 'markIncomplete'}
                    disabled={disabled}
                    onClick={onMarkIncomplete}
                >
                    {markIncompleteLabel}
                </Button>
            )}
            <Button
                variant="secondary"
                size="large"
                className={styles.actionButton}
                disabled={disabled}
                onClick={onEdit}
            >
                Редактировать
            </Button>
            <Button
                variant="secondary"
                size="large"
                className={styles.actionButton}
                loading={loadingAction === 'delete'}
                disabled={disabled}
                onClick={onDelete}
            >
                Удалить
            </Button>
        </div>
    );
}
