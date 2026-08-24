'use client';

import { useCallback, useMemo, useRef, useState, type JSX } from 'react';
import { Checkbox } from '@heroui/react';
import { notify } from '@/shared/lib/notifications';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { PageHeader } from '@/shared/ui/page-header';
import {
  useNotificationPreferences,
  useUpdateNotificationPreferences,
} from '@/features/notification-preferences';
import {
  buildChannelPreferencePayload,
  buildInitialChannelPreferences,
  channelPreferencesEqual,
  type NotificationChannelState,
} from '@/features/notification-preferences';
import { usePushSubscriptionStatus } from '@/features/push-notifications';
import { useSubscribePush } from '@/features/push-notifications';
import { isPushSupported } from '@/features/push-notifications';
import { NOTIFICATION_OPTIONS } from '@/features/notification-preferences';
import type { NotificationPreference } from '@/entities/user';
import styles from './NotificationSettings.module.css';

type NotificationSettingsViewProps = {
  readonly preferences: NotificationPreference[];
};

type GraceEmailRowProps = {
  readonly isSelected: boolean;
  readonly isDisabled?: boolean;
  readonly onChange?: (allowed: boolean) => void;
};

/** Единственная строка событий: подпись события + email-чекбокс. */
function GraceEmailRow({isSelected, isDisabled = false, onChange}: GraceEmailRowProps): JSX.Element {
  const graceOption = NOTIFICATION_OPTIONS[0];
  return (
    <div className={styles.eventRow}>
      <div className={styles.texts}>
        <span className={styles.label}>{graceOption.label}</span>
        <span className={styles.description}>{graceOption.description}</span>
      </div>
      <Checkbox
        isSelected={isSelected}
        onChange={onChange}
        isDisabled={isDisabled}
        aria-label={`${graceOption.label} — Email`}
        className={styles.checkbox}
      >
        <Checkbox.Content className={styles.checkboxContent}>
          <Checkbox.Control className={styles.checkboxControl}>
            <Checkbox.Indicator className={styles.checkboxIndicator}/>
          </Checkbox.Control>
        </Checkbox.Content>
      </Checkbox>
    </div>
  );
}

function NotificationSettingsView({ preferences }: NotificationSettingsViewProps): JSX.Element {
  const updateNotificationPreferences = useUpdateNotificationPreferences();
  const { refresh: refreshPushStatus, ...pushStatus } = usePushSubscriptionStatus();
  const { subscribe: subscribePush } = useSubscribePush();

  // Push is unavailable when the browser cannot receive push, the user has not
  // granted permission, or no subscription exists yet. The warning block
  // surfaces a one-tap enable action; the per-event push flag itself is not
  // exposed — the minimal screen (ticket #439) carries it from the server.
  const pushUnavailable =
    !pushStatus.isPending &&
    (pushStatus.isUnsupported || pushStatus.needsPermission || pushStatus.needsSubscription);

  // Checkbox edits live only as an override layer over the server state:
  // while null, the view follows fresh query data; once the user touches the
  // checkbox the override wins until that save settles.
  const [editedPrefs, setEditedPrefs] = useState<NotificationChannelState | null>(null);
  const serverPrefs = useMemo(() => buildInitialChannelPreferences(preferences), [preferences]);
  const notificationPrefs = editedPrefs ?? serverPrefs;

  // Ref mirrors for the async settle callbacks, which would otherwise see
  // stale closures: latestPrefsRef holds the newest desired state,
  // inFlightRef marks that a PUT is in the air.
  const latestPrefsRef = useRef<NotificationChannelState | null>(null);
  const inFlightRef = useRef(false);

  // Single-flight save: at most one PUT in the air, so the server never gets
  // a race of competing snapshots and the checkbox never has to be disabled.
  // Clicks made during a flight only update the override and latestPrefsRef;
  // the loop keeps re-sending the newest snapshot until the desired state
  // stops moving (trailing sync after each settle).
  const sync = useCallback(
    (snapshot: NotificationChannelState) => {
      inFlightRef.current = true;

      const run = async (): Promise<void> => {
        let current = snapshot;
        for (;;) {
          // Every save sends the full preference set; there is no submit
          // button and no success toast.
          const payload = buildChannelPreferencePayload(current);

          try {
            await updateNotificationPreferences.mutateAsync(payload);
            // The hook already wrote the response into the query cache, so
            // dropping the override changes nothing visually. If the user
            // clicked during the flight, the override stays — the trailing
            // iteration below owns it now.
            if (
              latestPrefsRef.current &&
              channelPreferencesEqual(latestPrefsRef.current, current)
            ) {
              latestPrefsRef.current = null;
              setEditedPrefs(null);
            }
          } catch (error: unknown) {
            notify.scenarios.profile.notificationPreferencesSaveError(error);
            // Roll the whole override back to the server state — after a
            // failed save the UI shows what is actually saved.
            latestPrefsRef.current = null;
            setEditedPrefs(null);
          }

          const latest = latestPrefsRef.current;
          if (!latest || channelPreferencesEqual(latest, current)) {
            break;
          }
          current = latest;
        }
        inFlightRef.current = false;
      };

      void run();
    },
    [updateNotificationPreferences],
  );

  const handleEmailChange = useCallback(
    (allowed: boolean) => {
      const base = editedPrefs ?? serverPrefs;
      const next: NotificationChannelState = {
        ...base,
        subscription_grace: { ...base.subscription_grace, email: allowed },
      };
      latestPrefsRef.current = next;
      setEditedPrefs(next);

      if (!inFlightRef.current) {
        sync(next);
      }
    },
    [editedPrefs, serverPrefs, sync],
  );

  const [isEnablingPush, setIsEnablingPush] = useState(false);

  const handleEnablePush = useCallback(async () => {
    if (!isPushSupported()) return;
    setIsEnablingPush(true);
    try {
      const outcome = await subscribePush();
      if (outcome.outcome === 'subscribed' || outcome.outcome === 'already-subscribed') {
        notify.scenarios.profile.pushEnabled();
        // Re-probe so the banner and hint update without a reload.
        refreshPushStatus();
      } else if (outcome.outcome === 'ios-needs-install') {
        notify.scenarios.profile.pushIosNeedsInstall();
      } else if (outcome.outcome === 'denied') {
        notify.scenarios.profile.pushPermissionDenied();
      } else if (outcome.outcome !== 'unsupported') {
        notify.scenarios.profile.pushEnableError(new Error(outcome.reason));
      }
    } catch (error) {
      notify.scenarios.profile.pushEnableError(error);
    } finally {
      setIsEnablingPush(false);
    }
  }, [subscribePush, refreshPushStatus]);

  // After the user has changed the permission via browser settings, the only
  // reliable way to pick up the new state is a reload — `Notification.permission`
  // is cached for the lifetime of the document. Browsers do not expose an API
  // to open site settings directly.
  const handleRecheckPermission = useCallback(() => {
    if (typeof window !== 'undefined') {
      window.location.reload();
    }
  }, []);

  return (
    <div className={styles.container}>
      <p className={styles.notificationsHint}>
        Письма об оплате подписки приходят на вашу почту{pushStatus.isReady ? ' и устройство' : ''}.
      </p>
      {pushUnavailable && (
        <div className={styles.pushWarning}>
          <p className={styles.pushWarningText}>
            {pushStatus.isUnsupported
              ? 'Пуши не поддерживаются этим браузером.'
              : pushStatus.permissionDenied
                ? 'Уведомления отключены в настройках браузера. Включите их для сайта, затем нажмите «Проверить разрешение».'
                : pushStatus.needsPermission
                  ? 'Разрешите уведомления в браузере, чтобы получать пуши.'
                  : 'Подпишитесь на пуши, чтобы получать уведомления на устройство.'}
          </p>
          {!pushStatus.isUnsupported &&
            (pushStatus.permissionDenied ? (
              <Button variant="secondary" size="medium" onClick={handleRecheckPermission}>
                Проверить разрешение
              </Button>
            ) : (
              <Button
                variant="secondary"
                size="medium"
                loading={isEnablingPush}
                onClick={() => void handleEnablePush()}
              >
                Разрешить пуши
              </Button>
            ))}
        </div>
      )}
      <GraceEmailRow
        isSelected={notificationPrefs.subscription_grace.email}
        onChange={handleEmailChange}
      />
    </div>
  );
}

export function NotificationSettings(): JSX.Element {
  const { data, isPending, isError, refetch } = useNotificationPreferences();

  return (
    <>
      <PageHeader title="Уведомления" backHref={ROUTES.profile} />
      {isError && (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить данные</p>
          <Button onClick={() => void refetch()} variant="secondary">
            Повторить
          </Button>
        </div>
      )}
      {!isError && isPending && (
        <div className={styles.container}>
          <p className={styles.notificationsHint}>
            Письма об оплате подписки приходят на вашу почту.
          </p>
          <GraceEmailRow isSelected={false} isDisabled />
        </div>
      )}
      {!isError && !isPending && (
        <NotificationSettingsView preferences={data} />
      )}
    </>
  );
}
