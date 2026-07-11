'use client';

import { useCallback, useState, type ChangeEvent, type FormEvent, type JSX } from 'react';
import { notify } from '@/shared/lib/toast';
import { ROUTES } from '@/shared/config/routes';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { PageHeader } from '@/shared/ui/page-header';
import { useMe } from '@/features/auth/api/hooks';
import { useUpdateMe } from '@/features/profile/api/hooks';
import type { User, UserUpdateCommand } from '@/entities/user/model/types';
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
  const [isEmailTouched, setIsEmailTouched] = useState(false);
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);

  const isEmailValid = email === '' || EMAIL_REGEX.test(email);

  const canSubmit = isEmailValid && !updateMe.isPending;

  const duplicateEmailError =
    submitError === 'duplicate_email'
      ? 'Этот email уже используется другим пользователем'
      : undefined;

  const emailError = (isSubmitAttempted || isEmailTouched) && !isEmailValid
    ? 'Введите корректный email'
    : duplicateEmailError;

  const handleEmailChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setIsEmailTouched(true);
    setEmail(event.currentTarget.value);
    setSubmitError(undefined);
  }, []);

  const handleSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
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

      updateMe.mutate(payload, {
        onSuccess: () => {
          setSubmitError(undefined);
          notify.success('Данные сохранены');
        },
        onError: (error) => {
          if (error.status === 409 && /почта уже используется/i.test(error.detail ?? '')) {
            setSubmitError('duplicate_email');
          } else {
            setSubmitError(error.detail);
          }
        },
      });
    },
    [canSubmit, updateMe, name, surname, patronymic, email, me],
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

      {submitError && submitError !== 'duplicate_email' && (
        <p className={styles.errorMessage} role="alert">
          {submitError}
        </p>
      )}

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={updateMe.isPending}
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

  return (
    <>
      <PageHeader title="Мои данные" backHref={ROUTES.profile} />
      {isMeError && (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить данные</p>
          <Button onClick={() => refetch()} variant="secondary">
            Повторить
          </Button>
        </div>
      )}
      {!isMeError && (isMeLoading || !me) && (
        <form className={styles.form}>
          <div className={styles.fields}>
            <TextField label="Фамилия" placeholder=" " value="" disabled fullWidth />
            <TextField label="Имя" placeholder=" " value="" disabled fullWidth />
            <TextField label="Отчество" placeholder=" " value="" disabled fullWidth />
            <TextField label="Email" placeholder="email@example.com" value="" disabled fullWidth />
          </div>
          <div className={styles.actions}>
            <Button type="submit" variant="primary" size="large" fullWidth disabled>
              Сохранить
            </Button>
          </div>
        </form>
      )}
      {!isMeError && !isMeLoading && me && <PersonalDataFormView key={me.id} me={me} />}
    </>
  );
}
