'use client';

import type {JSX, ReactNode} from 'react';
import NextLink from 'next/link';
import {BadgeDanger, BadgeGood, BadgeInfo, BadgeWarning, Sync} from '@/shared/assets/icons';
import type {NotificationAction} from '@/shared/lib/notifications/types';
import {buttonVariants, Button} from '@/shared/ui/design';
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
            // Спиннер — канонная иконка Sync в спине (паттерн loading
            // канонного Button): цвет наследует варианту тоста.
            return <Sync className="h-6 w-6 animate-spin" aria-hidden/>;
        case 'default':
            return null;
    }
}

/** Действие тоста на каноне (легаси Button/LinkButton снесены, #901):
 * onPress — канонная кнопка Clear, href — ссылка-кнопка
 * NextLink+buttonVariants (next/link не дружит с Radix Slot — прецедент
 * канонного button.tsx); оба закрывают тост по клику. */
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
            <NextLink
                href={action.href}
                className={buttonVariants({variant: 'primary', size: 'small', className: styles.action})}
                onClick={() => closeToast?.()}
            >
                {action.label}
            </NextLink>
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