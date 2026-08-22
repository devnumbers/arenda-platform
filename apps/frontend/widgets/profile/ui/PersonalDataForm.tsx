'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/notifications';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { PageHeader } from '@/shared/ui/page-header';
import { useMe } from '@/features/auth';
import { useUpdateMe } from '@/features/profile';
import type { User, UserUpdateCommand } from '@/entities/user';
import { TimezoneSelect } from './TimezoneSelect';
import styles from './PersonalDataForm.module.css';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type PersonalDataFormViewProps = {
  readonly me: User;
};

function PersonalDataFormView({ me }: PersonalDataFormViewProps): JSX.Element {
  const updateMe = useUpdateMe();

  const [surname, setSurname] = useState(me.surname ?? '');
  const [name, setName] = useState(me.name ?? '');
  const [patronymic, setPatronymic] = useState(me.patronymic ?? '');
  const [email, setEmail] = useState(me.email ?? '');
  const [timezone, setTimezone] = useState(me.timezone ?? '');
  const [isEmailTouched, setIsEmailTouched] = useState(false);
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

  const isEmailValid = email === '' || EMAIL_REGEX.test(email);

  const hasPersonalChanges = useMemo(
    () =>
      surname.trim() !== (me.surname ?? '') ||
      name.trim() !== (me.name ?? '') ||
      patronymic.trim() !== (me.patronymic ?? '') ||
      email.trim() !== (me.email ?? '') ||
      timezone !== (me.timezone ?? ''),
    [surname, name, patronymic, email, timezone, me],
  );

  const isSubmitting = updateMe.isPending;

  const canSubmit = isEmailValid && !isSubmitting && hasPersonalChanges;

  const emailError = (isSubmitAttempted || isEmailTouched) && !isEmailValid
    ? 'Введите корректный email'
    : undefined;

  const handleEmailChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setIsEmailTouched(true);
    setEmail(event.currentTarget.value);
  }, []);

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
      const initialTimezone = me.timezone ?? '';

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
      if (timezone !== initialTimezone) {
        payload.timezone = timezone || null;
      }

      if (Object.keys(payload).length === 0) {
        return;
      }

      try {
        await updateMe.mutateAsync(payload);
        notify.scenarios.profile.personalDataSaved();
      } catch (error) {
        notify.scenarios.profile.personalDataSaveError(error);
      }
    },
    [canSubmit, updateMe, name, surname, patronymic, email, timezone, me],
  );

  return (
    <form onSubmit={(event) => void handleSubmit(event)} className={styles.form}>
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
        <TimezoneSelect
          value={timezone}
          onChange={setTimezone}
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
  const { data: me, isPending, isError, refetch } = useMe();
  const updateMe = useUpdateMe();
  const autoTzSent = useRef(false);

  // Auto-detect the browser timezone and save it to the profile silently,
  // but only when there is no saved timezone yet (don't override manual
  // settings during trips). Runs once per mount.
  useEffect(() => {
    if (!me || autoTzSent.current) {
      return;
    }
    if (me.timezone) {
      autoTzSent.current = true;
      return;
    }
    if (updateMe.isPending) {
      return;
    }
    const detectedTz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (!detectedTz) {
      autoTzSent.current = true;
      return;
    }
    autoTzSent.current = true;
    updateMe.mutate({ timezone: detectedTz });
  }, [me, updateMe]);

  return (
    <>
      <PageHeader title="Мои данные" backHref={ROUTES.profile} />
      {isError && (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить данные</p>
          <Button onClick={() => void refetch()} variant="secondary">
            Повторить
          </Button>
        </div>
      )}
      {!isError && (isPending || !me) && (
        <form className={styles.form}>
          <div className={styles.fields}>
            <TextField label="Фамилия" placeholder=" " value="" disabled fullWidth />
            <TextField label="Имя" placeholder=" " value="" disabled fullWidth />
            <TextField label="Отчество" placeholder=" " value="" disabled fullWidth />
            <TextField label="Email" placeholder="email@example.com" value="" disabled fullWidth />
            <TextField label="Часовой пояс" placeholder=" " value="" disabled fullWidth />
          </div>
          <div className={styles.actions}>
            <Button type="submit" variant="primary" size="large" fullWidth disabled>
              Сохранить
            </Button>
          </div>
        </form>
      )}
      {!isError && !isPending && me && (
        <PersonalDataFormView key={me.id} me={me} />
      )}
    </>
  );
}
