'use client';

import type {JSX, ReactNode} from 'react';
import {Spinner} from '@heroui/react';
import {BadgeDanger, BadgeGood, BadgeInfo, BadgeWarning} from '@/shared/assets/icons';
import type {NotificationAction} from '@/shared/lib/notifications/types';
import {Button} from '@/shared/ui/button';
import {LinkButton} from '@/shared/ui/link-button';
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
    closeToast?: () => void;
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
                         closeToast,
                     }: {
    action: NotificationAction;
    closeToast?: () => void;
}): JSX.Element | null {
    if (action.onPress) {
        return (
            <Button
                variant="clear"
                size="small"
                className={styles.action}
                onClick={() => {
                    action.onPress?.();
                    closeToast?.();
                }}
            >
                {action.label}
            </Button>
        );
    }

    if (action.href) {
        return (
            <LinkButton
                href={action.href}
                variant='primary'
                size="small"
                className={styles.action}
                onClick={() => closeToast?.()}
            >
                {action.label}
            </LinkButton>
        );
    }

    return null;
}

export function ToastBody({
                              variant,
                              title,
                              description,
                              action,
                              closeToast,
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
            {action && <ToastAction action={action} closeToast={closeToast}/>}
        </div>
    );
}