'use client';

import { useCallback, useMemo, useState, type ChangeEvent, type FormEvent, type JSX } from 'react';
import { Checkbox } from '@heroui/react';
import { notify } from '@/shared/lib/notifications';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { PageHeader } from '@/shared/ui/page-header';
import { useMe } from '@/features/auth/api/hooks';
import {
  useNotificationPreferences,
  useUpdateMe,
  useUpdateNotificationPreferences,
} from '@/features/profile/api/hooks';
import type {
  NotificationEventType,
  NotificationPreference,
  User,
  UserUpdateCommand,
} from '@/entities/user/model/types';
import styles from './PersonalDataForm.module.css';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const NOTIFICATION_OPTIONS: {
  readonly eventType: NotificationEventType;
  readonly label: string;
}[] = [
  { eventType: 'operation_due', label: 'Напоминания об операциях' },
  { eventType: 'operation_overdue', label: 'Просроченные операции' },
  { eventType: 'lease_expiring', label: 'Окончание аренды' },
  { eventType: 'lease_requires_action', label: 'Аренда требует действия' },
];

type NotificationPreferencesState = Record<NotificationEventType, boolean>;

function buildInitialPreferences(
  preferences: NotificationPreference[],
): NotificationPreferencesState {
  const state: NotificationPreferencesState = {
    operation_due: true,
    operation_overdue: true,
    lease_expiring: true,
    lease_requires_action: true,
  };
  for (const preference of preferences) {
    state[preference.eventType] = preference.allowed;
  }
  return state;
}

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
  const [notificationPrefs, setNotificationPrefs] =
    useState<NotificationPreferencesState>(() =>
      buildInitialPreferences(preferences),
    );
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

  const hasPreferencesChanges = useMemo(() => {
    const initial = buildInitialPreferences(preferences);
    return NOTIFICATION_OPTIONS.some(
      ({ eventType }) => notificationPrefs[eventType] !== initial[eventType],
    );
  }, [notificationPrefs, preferences]);

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
      setNotificationPrefs((previous) => ({ ...previous, [eventType]: allowed }));
    },
    [],
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
        <div className={styles.notificationsList}>
          {NOTIFICATION_OPTIONS.map(({ eventType, label }) => (
            <Checkbox
              key={eventType}
              isSelected={notificationPrefs[eventType]}
              onChange={(allowed) => handleNotificationChange(eventType, allowed)}
              isDisabled={isSubmitting}
              className={styles.checkbox}
            >
              <Checkbox.Content className={styles.checkboxContent}>
                <Checkbox.Control className={styles.checkboxControl}>
                  <Checkbox.Indicator className={styles.checkboxIndicator} />
                </Checkbox.Control>
                <span>{label}</span>
              </Checkbox.Content>
            </Checkbox>
          ))}
        </div>
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
            <div className={styles.notificationsList}>
              {NOTIFICATION_OPTIONS.map(({ eventType, label }) => (
                <Checkbox key={eventType} isDisabled className={styles.checkbox}>
                  <Checkbox.Content className={styles.checkboxContent}>
                    <Checkbox.Control className={styles.checkboxControl}>
                      <Checkbox.Indicator className={styles.checkboxIndicator} />
                    </Checkbox.Control>
                    <span>{label}</span>
                  </Checkbox.Content>
                </Checkbox>
              ))}
            </div>
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
