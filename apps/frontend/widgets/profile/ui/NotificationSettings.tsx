'use client';

import { useCallback, useMemo, useRef, useState, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { PageHeader } from '@/shared/ui/page-header';
import {
  useNotificationPreferences,
  useUpdateNotificationPreferences,
} from '@/features/notification-preferences/api/hooks';
import {
  buildInitialPreferences,
  NOTIFICATION_OPTIONS,
  type NotificationPreferencesState,
} from '@/features/notification-preferences/lib/preferences';
import { NotificationPreferencesFields } from '@/features/notification-preferences/ui/NotificationPreferencesFields';
import type {
  NotificationEventType,
  NotificationPreference,
} from '@/entities/user/model/types';
import styles from './NotificationSettings.module.css';

type NotificationSettingsViewProps = {
  readonly preferences: NotificationPreference[];
};

// Equality over the fixed option set; used to tell whether the desired state
// moved on while a save was in flight.
function preferencesEqual(
  a: NotificationPreferencesState,
  b: NotificationPreferencesState,
): boolean {
  return NOTIFICATION_OPTIONS.every(
    (option) => a[option.eventType] === b[option.eventType],
  );
}

function NotificationSettingsView({ preferences }: NotificationSettingsViewProps): JSX.Element {
  const updateNotificationPreferences = useUpdateNotificationPreferences();

  // Checkbox edits live only as an override layer over the server state:
  // while null, the view follows fresh query data (e.g. the onboarding modal
  // saved over this open page); once the user touches a checkbox the override
  // wins until that save settles.
  const [editedPrefs, setEditedPrefs] = useState<NotificationPreferencesState | null>(null);
  const serverPrefs = useMemo(() => buildInitialPreferences(preferences), [preferences]);
  const notificationPrefs = editedPrefs ?? serverPrefs;

  // Ref mirrors for the async settle callbacks, which would otherwise see
  // stale closures: latestPrefsRef holds the newest desired state,
  // inFlightRef marks that a PUT is in the air.
  const latestPrefsRef = useRef<NotificationPreferencesState | null>(null);
  const inFlightRef = useRef(false);

  // Single-flight save: at most one PUT in the air, so the server never gets
  // a race of competing snapshots and the checkboxes never have to be
  // disabled. Clicks made during a flight only update the override and
  // latestPrefsRef; the loop keeps re-sending the newest snapshot until the
  // desired state stops moving (trailing sync after each settle).
  const sync = useCallback(
    (snapshot: NotificationPreferencesState) => {
      inFlightRef.current = true;

      const run = async (): Promise<void> => {
        let current = snapshot;
        for (;;) {
          // Every save sends the full preference set; there is no submit
          // button and no success toast.
          const payload: NotificationPreference[] = NOTIFICATION_OPTIONS.map(
            ({ eventType }) => ({
              eventType,
              allowed: current[eventType],
            }),
          );

          try {
            await updateNotificationPreferences.mutateAsync(payload);
            // The hook already wrote the response into the query cache, so
            // dropping the override changes nothing visually. If the user
            // clicked during the flight, the override stays — the trailing
            // iteration below owns it now.
            if (
              latestPrefsRef.current &&
              preferencesEqual(latestPrefsRef.current, current)
            ) {
              latestPrefsRef.current = null;
              setEditedPrefs(null);
            }
          } catch (error: unknown) {
            notify.scenarios.profile.notificationPreferencesSaveError(error);
            // Roll the whole override back to the server state, including
            // any clicks made during the failed request — a deliberate
            // simple behavior: after a failed save the UI shows what is
            // actually saved rather than a mix of saved and unsaved toggles.
            latestPrefsRef.current = null;
            setEditedPrefs(null);
          }

          const latest = latestPrefsRef.current;
          if (!latest || preferencesEqual(latest, current)) {
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

  const handleNotificationChange = useCallback(
    (eventType: NotificationEventType, allowed: boolean) => {
      const next: NotificationPreferencesState = {
        ...(editedPrefs ?? serverPrefs),
        [eventType]: allowed,
      };
      latestPrefsRef.current = next;
      setEditedPrefs(next);

      if (!inFlightRef.current) {
        sync(next);
      }
    },
    [editedPrefs, serverPrefs, sync],
  );

  return (
    <div className={styles.container}>
      <p className={styles.notificationsHint}>
        Напоминания приходят на вашу почту.
      </p>
      <NotificationPreferencesFields
        value={notificationPrefs}
        onChange={handleNotificationChange}
      />
    </div>
  );
}

export function NotificationSettings(): JSX.Element {
  const { data: preferences, isPending, isError, refetch } = useNotificationPreferences();

  return (
    <>
      <PageHeader title="Уведомления" backHref={ROUTES.profile} />
      {isError && (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить данные</p>
          <Button onClick={() => refetch()} variant="secondary">
            Повторить
          </Button>
        </div>
      )}
      {!isError && (isPending || !preferences) && (
        <div className={styles.container}>
          <p className={styles.notificationsHint}>
            Напоминания приходят на вашу почту.
          </p>
          <NotificationPreferencesFields disabled />
        </div>
      )}
      {!isError && !isPending && preferences && (
        <NotificationSettingsView preferences={preferences} />
      )}
    </>
  );
}
