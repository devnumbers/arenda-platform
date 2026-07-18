'use client';

import type {JSX} from 'react';
import {Checkbox} from '@heroui/react';
import type {NotificationEventType} from '@/entities/user/model/types';
import {
    NOTIFICATION_OPTIONS,
    type NotificationPreferencesState,
} from '@/features/notification-preferences/lib/preferences';
import styles from './NotificationPreferencesFields.module.css';

const EMPTY_PREFERENCES: NotificationPreferencesState = {
    operation_due: false,
    operation_overdue: false,
    lease_expiring: false,
    lease_requires_action: false,
};

export type NotificationPreferencesFieldsProps = {
    readonly value?: NotificationPreferencesState;
    readonly onChange?: (eventType: NotificationEventType, allowed: boolean) => void;
    readonly disabled?: boolean;
};

export function NotificationPreferencesFields({
                                                  value = EMPTY_PREFERENCES,
                                                  onChange,
                                                  disabled = false,
                                              }: NotificationPreferencesFieldsProps): JSX.Element {
    return (
        <div className={styles.list}>
            {NOTIFICATION_OPTIONS.map(({eventType, label, description}) => (
                <Checkbox
                    key={eventType}
                    isSelected={value[eventType]}
                    onChange={
                        onChange ? (allowed) => onChange(eventType, allowed) : undefined
                    }
                    isDisabled={disabled}
                    className={styles.checkbox}
                >
                    <Checkbox.Content className={styles.checkboxContent}>
                        <Checkbox.Control className={styles.checkboxControl}>
                            <Checkbox.Indicator className={styles.checkboxIndicator}/>
                        </Checkbox.Control>
                        <span className={styles.texts}>
              <span className={styles.label}>{label}</span>
              <span className={styles.description}>{description}</span>
            </span>
                    </Checkbox.Content>
                </Checkbox>
            ))}
        </div>
    );
}
