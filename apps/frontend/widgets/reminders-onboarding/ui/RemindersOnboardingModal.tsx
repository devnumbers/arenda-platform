'use client';

import {type JSX, useCallback, useEffect, useState} from 'react';
import {Modal} from '@heroui/react';
import {Drawer} from '@heroui/react/drawer';
import {notify} from '@/shared/lib/notifications';
import {reportClientError} from '@/shared/lib/error-reporting/report-client-error';
import {Bell} from '@/shared/assets/icons';
import {Icon} from '@/shared/ui/icon';
import {Button} from '@/shared/ui/button';
import {REMINDERS_ONBOARDING_POPUP_KEY, useMarkPopupSeen, usePendingPopups,} from '@/features/popups/api/hooks';
import {
    useNotificationPreferences,
    useUpdateNotificationPreferences,
} from '@/features/notification-preferences/api/hooks';
import {
    buildInitialPreferences,
    buildPreferencePayload,
    type NotificationPreferencesState,
} from '@/features/notification-preferences/lib/preferences';
import {NotificationPreferencesFields} from '@/features/notification-preferences/ui/NotificationPreferencesFields';
import type {NotificationEventType, NotificationPreference,} from '@/entities/user/model/types';
import styles from './RemindersOnboardingModal.module.css';

const MOBILE_MEDIA_QUERY = '(max-width: 767px)';

// null until the media query is read on the client — the overlay type
// (Modal vs Drawer) is chosen only afterwards, so there is no flash of the
// wrong container.
function useIsMobile(): boolean | null {
    const [isMobile, setIsMobile] = useState<boolean | null>(null);

    useEffect(() => {
        const mql = window.matchMedia(MOBILE_MEDIA_QUERY);
        const update = (matches: boolean) => setIsMobile(matches);

        update(mql.matches);

        const onChange = (event: MediaQueryListEvent) => update(event.matches);
        mql.addEventListener('change', onChange);
        return () => mql.removeEventListener('change', onChange);
    }, []);

    return isMobile;
}

type OnboardingContentProps = {
    readonly preferences: NotificationPreferencesState;
    readonly isSubmitting: boolean;
    readonly onPreferenceChange: (eventType: NotificationEventType, allowed: boolean) => void;
    readonly onSave: () => void;
};

function OnboardingContent({
                               preferences,
                               isSubmitting,
                               onPreferenceChange,
                               onSave,
                           }: OnboardingContentProps): JSX.Element {
    return (
        <>
            <div className={styles.header}>
                <span className={styles.bell}>
                    <Icon size="l">
                        <Bell/>
                    </Icon>
                </span>
                <h2 className={styles.heading}>Напоминания</h2>
            </div>
            <div className={styles.content}>
                <p className={styles.intro}>
                    Напомним о важных датах по аренде на вашу почту. Выберите, какие напоминания получать:
                </p>
                <NotificationPreferencesFields
                    value={preferences}
                    onChange={onPreferenceChange}
                    disabled={isSubmitting}
                />
                <p className={styles.hint}>
                    Изменить выбор можно в любой момент: Профиль → Уведомления
                </p>
            </div>
            <div className={styles.footer}>
                <Button
                    variant="primary"
                    size="large"
                    fullWidth
                    loading={isSubmitting}
                    onClick={onSave}
                >
                    Сохранить
                </Button>
            </div>
        </>
    );
}

type RemindersOnboardingModalContentProps = {
    readonly preferences: NotificationPreference[];
};

function RemindersOnboardingModalContent({
                                             preferences,
                                         }: RemindersOnboardingModalContentProps): JSX.Element | null {
    const updateNotificationPreferences = useUpdateNotificationPreferences();
    const markPopupSeen = useMarkPopupSeen();

    const [isOpen, setIsOpen] = useState(true);
    const [notificationPrefs, setNotificationPrefs] =
        useState<NotificationPreferencesState>(() =>
            buildInitialPreferences(preferences),
        );

    const isMobile = useIsMobile();

    const isSubmitting =
        updateNotificationPreferences.isPending || markPopupSeen.isPending;

    const handlePreferenceChange = useCallback(
        (eventType: NotificationEventType, allowed: boolean) => {
            setNotificationPrefs((previous) => ({...previous, [eventType]: allowed}));
        },
        [],
    );

    const handleSave = useCallback(async () => {
        const payload: NotificationPreference[] = buildPreferencePayload(
            notificationPrefs,
            preferences,
        );

        try {
            await updateNotificationPreferences.mutateAsync(payload);
        } catch (error) {
            notify.scenarios.profile.notificationPreferencesSaveError(error);
            return;
        }

        try {
            await markPopupSeen.mutateAsync(REMINDERS_ONBOARDING_POPUP_KEY);
        } catch (error) {
            // A one-off repeat of the popup on the next visit is acceptable.
            reportClientError(
                'Failed to mark reminders onboarding popup as seen',
                error instanceof Error ? error.stack : undefined,
            );
        }

        setIsOpen(false);
    }, [markPopupSeen, notificationPrefs, updateNotificationPreferences]);

    // Wait for the media query to resolve, then render exactly one overlay:
    // two open overlays would mean two focus traps fighting each other.
    if (isMobile === null) {
        return null;
    }

    const content = (
        <OnboardingContent
            preferences={notificationPrefs}
            isSubmitting={isSubmitting}
            onPreferenceChange={handlePreferenceChange}
            onSave={handleSave}
        />
    );

    // The only way out is "Сохранить": no close trigger is rendered,
    // backdrop click and Esc are disabled on the backdrop.
    if (isMobile) {
        return (
            <Drawer isOpen={isOpen}>
                <Drawer.Backdrop isDismissable={false} isKeyboardDismissDisabled>
                    <Drawer.Content placement="bottom">
                        <Drawer.Dialog aria-label="Напоминания" className={styles.sheet}>
                            <Drawer.Handle className={styles.handle}/>
                            {content}
                        </Drawer.Dialog>
                    </Drawer.Content>
                </Drawer.Backdrop>
            </Drawer>
        );
    }

    return (
        <Modal isOpen={isOpen}>
            <Modal.Backdrop isDismissable={false} isKeyboardDismissDisabled>
                <Modal.Container placement="center" size="md">
                    <Modal.Dialog aria-label="Напоминания" className={styles.dialog}>
                        {content}
                    </Modal.Dialog>
                </Modal.Container>
            </Modal.Backdrop>
        </Modal>
    );
}

export function RemindersOnboardingModal(): JSX.Element | null {
    const {data: pendingPopups, isPending: isPopupsPending} = usePendingPopups();
    const {data: preferences, isPending: isPreferencesPending} =
        useNotificationPreferences();

    if (isPopupsPending || isPreferencesPending) {
        return null;
    }

    // If a request failed after the react-query retries, its data stays
    // undefined and the one-time onboarding is silently skipped until the next
    // page load — an accepted trade-off.
    if (!pendingPopups?.includes(REMINDERS_ONBOARDING_POPUP_KEY) || !preferences) {
        return null;
    }

    return <RemindersOnboardingModalContent preferences={preferences}/>;
}
