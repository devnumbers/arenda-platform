'use client';

import { useCallback, useMemo, useState, type ChangeEvent, type FormEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { PageHeader } from '@/shared/ui/page-header';
import { useMe } from '@/features/auth/api/hooks';
import { useUpdateMe } from '@/features/profile/api/hooks';
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
  User,
  UserUpdateCommand,
} from '@/entities/user/model/types';
import styles from './PersonalDataForm.module.css';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type PersonalDataFormViewProps = {
  readonly me: User;
  readonly preferences: NotificationPreference[];
};

function PersonalDataFormView({ me, preferences }: PersonalDataFormViewProps): JSX.Element {
  const updateMe = useUpdateMe();
  const updateNotificationPreferences = useUpdateNotificationPreferences();

  const [surname, setSurname] = useState(me.surname ?? '');
  const [name, setName] = useState(me.name ?? '');
  const [patronymic, setPatronymic] = useState(me.patronymic ?? '');
  const [email, setEmail] = useState(me.email ?? '');
  // Checkbox edits live only as an override layer over the server state:
  // while null, the form follows fresh query data (e.g. the onboarding modal
  // saved over this open page); once the user touches a checkbox the override
  // wins until their own save succeeds. Comparing against the fresh server
  // `preferences` cannot tell "user edited" from "server changed", so an
  // explicit override is the only reliable dirty tracking.
  const [editedPrefs, setEditedPrefs] = useState<NotificationPreferencesState | null>(null);
  const serverPrefs = useMemo(() => buildInitialPreferences(preferences), [preferences]);
  const notificationPrefs = editedPrefs ?? serverPrefs;
  const [isEmailTouched, setIsEmailTouched] = useState(false);
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

  const isEmailValid = email === '' || EMAIL_REGEX.test(email);

  const hasPersonalChanges = useMemo(
    () =>
      surname.trim() !== (me.surname ?? '') ||
      name.trim() !== (me.name ?? '') ||
      patronymic.trim() !== (me.patronymic ?? '') ||
      email.trim() !== (me.email ?? ''),
    [surname, name, patronymic, email, me],
  );

  const hasPreferencesChanges = useMemo(
    () =>
      NOTIFICATION_OPTIONS.some(
        ({ eventType }) => notificationPrefs[eventType] !== serverPrefs[eventType],
      ),
    [notificationPrefs, serverPrefs],
  );

  const isSubmitting =
    updateMe.isPending || updateNotificationPreferences.isPending;

  const canSubmit =
    isEmailValid && !isSubmitting && (hasPersonalChanges || hasPreferencesChanges);

  const emailError = (isSubmitAttempted || isEmailTouched) && !isEmailValid
    ? 'Введите корректный email'
    : undefined;

  const handleEmailChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setIsEmailTouched(true);
    setEmail(event.currentTarget.value);
  }, []);

  const handleNotificationChange = useCallback(
    (eventType: NotificationEventType, allowed: boolean) => {
      setEditedPrefs((previous) => ({
        ...(previous ?? serverPrefs),
        [eventType]: allowed,
      }));
    },
    [serverPrefs],
  );

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setIsSubmitAttempted(true);

      if (!canSubmit) {
        return;
      }

      const initialSurname = me.surname ?? '';
      const initialName = me.name ?? '';
      const initialPatronymic = me.patronymic ?? '';
      const initialEmail = me.email ?? '';

      const payload: UserUpdateCommand = {};
      const trimmedSurname = surname.trim();
      const trimmedName = name.trim();
      const trimmedPatronymic = patronymic.trim();
      const trimmedEmail = email.trim();

      if (trimmedSurname !== initialSurname) {
        payload.surname = trimmedSurname || null;
      }
      if (trimmedName !== initialName) {
        payload.name = trimmedName || null;
      }
      if (trimmedPatronymic !== initialPatronymic) {
        payload.patronymic = trimmedPatronymic || null;
      }
      if (trimmedEmail !== initialEmail) {
        payload.email = trimmedEmail || null;
      }

      const preferencesPayload: NotificationPreference[] =
        NOTIFICATION_OPTIONS.map(({ eventType }) => ({
          eventType,
          allowed: notificationPrefs[eventType],
        }));

      const [personalResult, preferencesResult] = await Promise.allSettled([
        Object.keys(payload).length > 0
          ? updateMe.mutateAsync(payload)
          : Promise.resolve(),
        hasPreferencesChanges
          ? updateNotificationPreferences.mutateAsync(preferencesPayload)
          : Promise.resolve(),
      ]);

      if (personalResult.status === 'rejected') {
        notify.scenarios.profile.personalDataSaveError(personalResult.reason);
      }
      if (preferencesResult.status === 'rejected') {
        notify.scenarios.profile.notificationPreferencesSaveError(
          preferencesResult.reason,
        );
      }
      if (preferencesResult.status === 'fulfilled') {
        // The submitted values are now the server state; drop the override
        // layer so the form follows fresh query data again.
        setEditedPrefs(null);
      }
      if (
        personalResult.status === 'fulfilled' &&
        preferencesResult.status === 'fulfilled'
      ) {
        notify.scenarios.profile.personalDataSaved();
      }
    },
    [
      canSubmit,
      updateMe,
      updateNotificationPreferences,
      hasPreferencesChanges,
      name,
      surname,
      patronymic,
      email,
      notificationPrefs,
      me,
    ],
  );

  return (
    <form onSubmit={handleSubmit} className={styles.form}>
      <div className={styles.fields}>
        <TextField
          label="Фамилия"
          placeholder=" "
          value={surname}
          onChange={(event) => setSurname(event.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Имя"
          placeholder=" "
          value={name}
          onChange={(event) => setName(event.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Отчество"
          placeholder=" "
          value={patronymic}
          onChange={(event) => setPatronymic(event.currentTarget.value)}
          fullWidth
        />
        <TextField
          label="Email"
          placeholder="email@example.com"
          value={email}
          onChange={handleEmailChange}
          error={emailError}
          fullWidth
        />
      </div>

      <div className={styles.notifications}>
        <h2 className={styles.notificationsTitle}>Уведомления</h2>
        <p className={styles.notificationsHint}>
          Напоминания приходят на подтверждённый email.
        </p>
        <NotificationPreferencesFields
          value={notificationPrefs}
          onChange={handleNotificationChange}
          disabled={isSubmitting}
        />
      </div>

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={isSubmitting}
          disabled={!canSubmit}
        >
          Сохранить
        </Button>
      </div>
    </form>
  );
}

export function PersonalDataForm(): JSX.Element {
  const { data: me, isPending: isMeLoading, isError: isMeError, refetch } = useMe();
  const {
    data: preferences,
    isPending: isPreferencesLoading,
    isError: isPreferencesError,
    refetch: refetchPreferences,
  } = useNotificationPreferences();

  const isError = isMeError || isPreferencesError;
  const isLoading = isMeLoading || isPreferencesLoading;

  return (
    <>
      <PageHeader title="Мои данные" backHref={ROUTES.profile} />
      {isError && (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить данные</p>
          <Button
            onClick={() => {
              refetch();
              refetchPreferences();
            }}
            variant="secondary"
          >
            Повторить
          </Button>
        </div>
      )}
      {!isError && (isLoading || !me || !preferences) && (
        <form className={styles.form}>
          <div className={styles.fields}>
            <TextField label="Фамилия" placeholder=" " value="" disabled fullWidth />
            <TextField label="Имя" placeholder=" " value="" disabled fullWidth />
            <TextField label="Отчество" placeholder=" " value="" disabled fullWidth />
            <TextField label="Email" placeholder="email@example.com" value="" disabled fullWidth />
          </div>
          <div className={styles.notifications}>
            <h2 className={styles.notificationsTitle}>Уведомления</h2>
            <p className={styles.notificationsHint}>
              Напоминания приходят на вашу почту.
            </p>
            <NotificationPreferencesFields disabled />
          </div>
          <div className={styles.actions}>
            <Button type="submit" variant="primary" size="large" fullWidth disabled>
              Сохранить
            </Button>
          </div>
        </form>
      )}
      {!isError && !isLoading && me && preferences && (
        <PersonalDataFormView key={me.id} me={me} preferences={preferences} />
      )}
    </>
  );
}
