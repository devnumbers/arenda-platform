'use client';

import type {JSX, ReactNode} from 'react';
import {Spinner} from '@heroui/react';
import {BadgeDanger, BadgeGood, BadgeInfo, BadgeWarning} from '@/shared/assets/icons';
import type {NotificationAction} from '@/shared/lib/notifications/types';
import styles from './ToastProvider.module.css';

export type ToastVariant =
    | 'success'
    | 'error'
    | 'warning'
    | 'info'
    | 'loading'
    | 'default';

export type ToastBodyProps = {
    variant: ToastVariant;
    title: ReactNode;
    description?: ReactNode;
    action?: NotificationAction;
};

function VariantIcon({
                         variant,
                     }: {
    variant: ToastVariant;
}): JSX.Element | null {
    switch (variant) {
        case 'success':
            return <BadgeGood aria-hidden/>;
        case 'error':
            return <BadgeDanger aria-hidden/>;
        case 'warning':
            return <BadgeWarning aria-hidden/>;
        case 'info':
            return <BadgeInfo aria-hidden/>;
        case 'loading':
            return <Spinner color="current" size="sm"/>;
        default:
            return null;
    }
}

function ToastAction({
                         action,
                     }: {
    action: NotificationAction;
}): JSX.Element | null {
    if (action.onPress) {
        return (
            <button
                type="button"
                className={styles.actionButton}
                onClick={action.onPress}
            >
                {action.label}
            </button>
        );
    }

    if (action.href) {
        return (
            <a className={styles.actionButton} href={action.href}>
                {action.label}
            </a>
        );
    }

    return null;
}

export function ToastBody({
                              variant,
                              title,
                              description,
                              action,
                          }: ToastBodyProps): JSX.Element {
    return (
        <div className={styles.body} data-variant={variant}>
            {variant !== 'default' && (
                <div className={styles.icon}>
                    <VariantIcon variant={variant}/>
                </div>
            )}
            <div className={styles.content}>
                {!!title && <div className={styles.title}>{title}</div>}
                {!!description && <div className={styles.description}>{description}</div>}
            </div>
            {action && <ToastAction action={action}/>}
        </div>
    );
}