'use client';

import {type JSX, useCallback, useEffect, useState} from 'react';
import {Modal} from '@heroui/react';
import {Drawer} from '@heroui/react/drawer';
import {notify} from '@/shared/lib/notifications';
import {reportClientError} from '@/shared/lib/error-reporting/report-client-error';
import {Bell, BellOff} from '@/shared/assets/icons';
import {Icon} from '@/shared/ui/icon';
import {Button} from '@/shared/ui/button';
import {REMINDERS_ONBOARDING_POPUP_KEY, useMarkPopupSeen, usePendingPopups,} from '@/features/popups';
import {
    useNotificationPreferences,
    useUpdateNotificationPreferences,
} from '@/features/notification-preferences';
import {
    buildInitialPreferences,
    buildPreferencePayload,
    type NotificationPreferencesState,
} from '@/features/notification-preferences';
import {NotificationPreferencesFields} from '@/features/notification-preferences';
import {useSubscribePush} from '@/features/push-notifications';
import {isPushSupported} from '@/features/push-notifications';
import type {CarriedNotificationPreference, NotificationEventType, NotificationPreference,} from '@/entities/user';
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

type Step = 'types' | 'push';

type TypesContentProps = {
    readonly preferences: NotificationPreferencesState;
    readonly isSubmitting: boolean;
    readonly onPreferenceChange: (eventType: NotificationEventType, allowed: boolean) => void;
    readonly onNext: () => void;
};

function TypesContent({
                          preferences,
                          isSubmitting,
                          onPreferenceChange,
                          onNext,
                      }: TypesContentProps): JSX.Element {
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
                    onClick={onNext}
                >
                    Далее
                </Button>
            </div>
        </>
    );
}

type PushContentProps = {
    readonly isBusy: boolean;
    readonly onAllow: () => void;
    readonly onSkip: () => void;
};

function PushContent({isBusy, onAllow, onSkip}: PushContentProps): JSX.Element {
    return (
        <>
            <div className={styles.header}>
                <span className={styles.bell}>
                    <Icon size="l">
                        <BellOff/>
                    </Icon>
                </span>
                <h2 className={styles.heading}>Пуши на устройство</h2>
            </div>
            <div className={styles.content}>
                <p className={styles.intro}>
                    Получайте напоминания мгновенно на экран телефона. Запросим разрешение браузера — можно отказаться в любой момент.
                </p>
                <p className={styles.hint}>
                    На iPhone для пушей нужно сначала добавить приложение на экран «Домой».
                </p>
            </div>
            <div className={styles.footer}>
                <div className={styles.footerButtons}>
                    <Button
                        variant="primary"
                        size="large"
                        fullWidth
                        loading={isBusy}
                        onClick={onAllow}
                    >
                        Разрешить пуши
                    </Button>
                    <Button
                        variant="clear"
                        size="medium"
                        fullWidth
                        disabled={isBusy}
                        onClick={onSkip}
                    >
                        Пропустить
                    </Button>
                </div>
            </div>
        </>
    );
}

type RemindersOnboardingModalContentProps = {
    readonly preferences: NotificationPreference[];
    readonly carried: readonly CarriedNotificationPreference[];
};

function RemindersOnboardingModalContent({
                                             preferences,
                                             carried,
                                         }: RemindersOnboardingModalContentProps): JSX.Element | null {
    const updateNotificationPreferences = useUpdateNotificationPreferences();
    const markPopupSeen = useMarkPopupSeen();
    const {subscribe: subscribePush} = useSubscribePush();

    const [isOpen, setIsOpen] = useState(true);
    const [step, setStep] = useState<Step>('types');
    const [notificationPrefs, setNotificationPrefs] =
        useState<NotificationPreferencesState>(() =>
            buildInitialPreferences(preferences),
        );
    const [isPushBusy, setIsPushBusy] = useState(false);

    const isMobile = useIsMobile();

    const isSavingPreferences =
        updateNotificationPreferences.isPending || markPopupSeen.isPending;

    const handlePreferenceChange = useCallback(
        (eventType: NotificationEventType, allowed: boolean) => {
            setNotificationPrefs((previous) => ({...previous, [eventType]: allowed}));
        },
        [],
    );

    const closeWithPopupSeen = useCallback(async () => {
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
    }, [markPopupSeen]);

    const handleNext = useCallback(async () => {
        // carried: выведенные из продукта события сервер требует в PUT
        // полным набором (тикет #381) — переносятся как есть.
        const payload = buildPreferencePayload(
            notificationPrefs,
            preferences,
            carried,
        );

        try {
            await updateNotificationPreferences.mutateAsync(payload);
        } catch (error) {
            notify.scenarios.profile.notificationPreferencesSaveError(error);
            return;
        }

        // Skip the push step on browsers that cannot receive push at all —
        // there is nothing actionable for the user there.
        if (!isPushSupported()) {
            await closeWithPopupSeen();
            return;
        }

        setStep('push');
    }, [closeWithPopupSeen, notificationPrefs, preferences, carried, updateNotificationPreferences]);

    const handleAllowPush = useCallback(async () => {
        setIsPushBusy(true);
        try {
            const outcome = await subscribePush();
            if (outcome.outcome === 'subscribed' || outcome.outcome === 'already-subscribed') {
                notify.scenarios.profile.pushEnabled();
            } else if (outcome.outcome === 'ios-needs-install') {
                notify.scenarios.profile.pushIosNeedsInstall();
            } else if (outcome.outcome === 'denied') {
                notify.scenarios.profile.pushPermissionDenied();
            } else if (outcome.outcome === 'unsupported') {
                // Silently move on — push is unavailable on this browser.
            } else {
                notify.scenarios.profile.pushEnableError(new Error(outcome.reason));
            }
        } catch (error) {
            notify.scenarios.profile.pushEnableError(error);
        } finally {
            setIsPushBusy(false);
            await closeWithPopupSeen();
        }
    }, [closeWithPopupSeen, subscribePush]);

    const handleSkipPush = useCallback(() => {
        void closeWithPopupSeen();
    }, [closeWithPopupSeen]);

    // Wait for the media query to resolve, then render exactly one overlay:
    // two open overlays would mean two focus traps fighting each other.
    if (isMobile === null) {
        return null;
    }

    const content =
        step === 'types' ? (
            <TypesContent
                preferences={notificationPrefs}
                isSubmitting={isSavingPreferences}
                onPreferenceChange={handlePreferenceChange}
                onNext={handleNext}
            />
        ) : (
            <PushContent
                isBusy={isPushBusy}
                onAllow={handleAllowPush}
                onSkip={handleSkipPush}
            />
        );

    // The only way out is completing both steps: no close trigger is rendered,
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
    const {data: preferencesData, isPending: isPreferencesPending} =
        useNotificationPreferences();

    if (isPopupsPending || isPreferencesPending) {
        return null;
    }

    // If a request failed after the react-query retries, its data stays
    // undefined and the one-time onboarding is silently skipped until the next
    // page load — an accepted trade-off.
    if (!pendingPopups?.includes(REMINDERS_ONBOARDING_POPUP_KEY) || !preferencesData) {
        return null;
    }

    return (
        <RemindersOnboardingModalContent
            preferences={preferencesData.preferences}
            carried={preferencesData.carried}
        />
    );
}
