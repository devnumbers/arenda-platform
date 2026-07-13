'use client';

import {type JSX, type ReactNode} from 'react';
import {Clock} from '@/shared/assets/icons';
import {Button} from '@/shared/ui/button';
import {formatTimer} from '@/features/auth/lib/format-timer';
import styles from './SendCodeButton.module.css';

export type SendCodeButtonProps = {
    remainingSeconds: number;
    loading: boolean;
    disabled?: boolean;
    onClick?: () => void;
    type?: 'button' | 'submit';
    children: ReactNode;
    timerLabel?: (remainingSeconds: number) => ReactNode;
};

export function SendCodeButton({
                                   remainingSeconds,
                                   loading,
                                   disabled = false,
                                   onClick,
                                   type = 'button',
                                   children,
                                   timerLabel,
                               }: SendCodeButtonProps): JSX.Element {
    const label = remainingSeconds > 0 && timerLabel
        ? timerLabel(remainingSeconds)
        : children;
    return (
        <Button
            type={type}
            variant="primary"
            size="large"
            fullWidth
            disabled={disabled || remainingSeconds > 0}
            loading={loading}
            onClick={onClick}
            subtitle={
                remainingSeconds > 0 ? (
                    <span className={styles.timerRow}>
            <Clock className={styles.timerIcon}/>
            <span className={styles.timerText}>{formatTimer(remainingSeconds)}</span>
          </span>
                ) : undefined
            }
        >
            {label}
        </Button>
    );
}
