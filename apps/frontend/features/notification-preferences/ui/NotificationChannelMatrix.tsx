'use client';

import type {JSX} from 'react';
import {Checkbox} from '@heroui/react';
import type {NotificationEventType} from '@/entities/user';
import {
    NOTIFICATION_OPTIONS,
    type NotificationChannelState,
} from '@/features/notification-preferences/lib/preferences';
import styles from './NotificationChannelMatrix.module.css';

export type NotificationChannelMatrixProps = {
    readonly value?: NotificationChannelState;
    readonly onChangeEmail?: (eventType: NotificationEventType, allowed: boolean) => void;
    readonly onChangePush?: (eventType: NotificationEventType, allowed: boolean) => void;
    readonly pushUnavailable?: boolean;
    readonly disabled?: boolean;
};

const EMPTY_STATE: NotificationChannelState = {
    operation_due: {email: false, push: false},
    operation_overdue: {email: false, push: false},
    lease_expiring: {email: false, push: false},
    lease_requires_action: {email: false, push: false},
    subscription_grace: {email: false, push: false},
};

export function NotificationChannelMatrix({
                                             value = EMPTY_STATE,
                                             onChangeEmail,
                                             onChangePush,
                                             pushUnavailable = false,
                                             disabled = false,
                                         }: NotificationChannelMatrixProps): JSX.Element {
    return (
        <div className={styles.matrix} role="group" aria-label="Каналы уведомлений">
            <div className={styles.headerRow} aria-hidden>
                <span className={styles.typeHeader}/>
                <span className={styles.channelHeader}>Email</span>
                <span className={styles.channelHeader}>Пуши</span>
            </div>
            {NOTIFICATION_OPTIONS.map(({eventType, label, description}) => (
                <div className={styles.row} key={eventType}>
                    <div className={styles.texts}>
                        <span className={styles.label}>{label}</span>
                        <span className={styles.description}>{description}</span>
                    </div>
                    <Checkbox
                        isSelected={value[eventType].email}
                        onChange={
                            onChangeEmail ? (allowed) => onChangeEmail(eventType, allowed) : undefined
                        }
                        isDisabled={disabled}
                        aria-label={`${label} — Email`}
                        className={styles.checkbox}
                    >
                        <Checkbox.Content className={styles.checkboxContent}>
                            <Checkbox.Control className={styles.checkboxControl}>
                                <Checkbox.Indicator className={styles.checkboxIndicator}/>
                            </Checkbox.Control>
                        </Checkbox.Content>
                    </Checkbox>
                    <Checkbox
                        isSelected={value[eventType].push && !pushUnavailable}
                        onChange={
                            onChangePush && !pushUnavailable
                                ? (allowed) => onChangePush(eventType, allowed)
                                : undefined
                        }
                        isDisabled={disabled || pushUnavailable}
                        aria-label={`${label} — Пуши`}
                        className={styles.checkbox}
                    >
                        <Checkbox.Content className={styles.checkboxContent}>
                            <Checkbox.Control className={styles.checkboxControl}>
                                <Checkbox.Indicator className={styles.checkboxIndicator}/>
                            </Checkbox.Control>
                        </Checkbox.Content>
                    </Checkbox>
                </div>
            ))}
        </div>
    );
}
