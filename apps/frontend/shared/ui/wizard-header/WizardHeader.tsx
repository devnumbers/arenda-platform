'use client';

import type {JSX, ReactNode} from 'react';
import clsx from 'clsx';
import {IconButton} from '@/shared/ui/icon-button';
import {ArrowLeft, Cancel} from '@/shared/assets/icons';
import styles from './WizardHeader.module.css';

export interface WizardHeaderProps {
    title: ReactNode;
    step: number;
    totalSteps: number;
    onBack?: () => void;
    onCancel?: () => void;
    className?: string;
}

export function WizardHeader({
                                 title,
                                 step,
                                 totalSteps,
                                 onBack,
                                 onCancel,
                                 className,
                             }: WizardHeaderProps): JSX.Element {
    return (
        <header className={clsx(styles.header, className)}>
            <div className={styles.topRow}>
                {onBack && (
                    <IconButton
                        variant="secondary"
                        size="large"
                        icon={<ArrowLeft/>}
                        aria-label="Назад"
                        onClick={onBack}
                        className={styles.iconButton}
                    />
                )}
                <h1 className={styles.title}>{title}</h1>
                {onCancel && (
                    <IconButton
                        variant="secondary"
                        size="large"
                        icon={<Cancel/>}
                        aria-label="Отменить"
                        onClick={onCancel}
                        className={styles.iconButton}
                    />
                )}
            </div>
            {totalSteps > 0 && (
                <div className={styles.progressRow}>
                    <span className={styles.badge}>{step} из {totalSteps}</span>
                    <div
                        className={styles.progressBar}
                        role="progressbar"
                        aria-label={`Шаг ${step} из ${totalSteps}`}
                        aria-valuenow={step}
                        aria-valuemin={1}
                        aria-valuemax={totalSteps}
                    >
                        {Array.from({length: totalSteps}, (_, i) => (
                            <div
                                key={i}
                                className={clsx(styles.segment, i < step && styles.active)}
                            />
                        ))}
                    </div>
                </div>
            )}
        </header>
    );
}
