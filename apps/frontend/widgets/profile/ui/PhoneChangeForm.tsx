'use client';

import {
  useCallback,
  useState,
  type ChangeEvent,
  type FormEvent,
  type JSX,
} from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/shared/lib/notifications';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { PageHeader } from '@/shared/ui/page-header';
import { useMe } from '@/features/auth/api/hooks';
import {
  useChangePhone,
  useChangePhoneSendCode,
} from '@/features/profile/api/hooks';
import { formatPhoneInput, normalizePhone } from '@/shared/lib/phone';
import { ROUTES } from '@/shared/config/routes';
import styles from './PhoneChangeForm.module.css';

const CODE_LENGTH = 6;

function isValidPhone(formatted: string): boolean {
  return normalizePhone(formatted).length === 12;
}

function isValidCode(value: string): boolean {
  return /^\d{6}$/.test(value);
}

type Step = 'phone' | 'code';

function PhoneChangeFormView({ currentPhone }: { currentPhone: string }): JSX.Element {
  const router = useRouter();
  const sendCode = useChangePhoneSendCode();
  const changePhone = useChangePhone();

  const [step, setStep] = useState<Step>('phone');
  const [phone, setPhone] = useState('');
  const [code, setCode] = useState('');
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

  const normalizedPhone = normalizePhone(phone);
  const phoneError = isSubmitAttempted && !isValidPhone(phone)
    ? 'Введите корректный номер телефона'
    : undefined;
  const codeError = isSubmitAttempted && !isValidCode(code)
    ? 'Введите 6-значный код'
    : undefined;

  const isSamePhone = normalizedPhone === currentPhone;

  const handlePhoneChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    setPhone(formatPhoneInput(event.currentTarget.value));
  }, []);

  const handleCodeChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
    const value = event.currentTarget.value.replace(/\D/g, '').slice(0, CODE_LENGTH);
    setCode(value);
  }, []);

  const handleSendCode = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setIsSubmitAttempted(true);

      if (!isValidPhone(phone) || isSamePhone) {
        return;
      }

      sendCode.mutate(
        { phone: normalizedPhone },
        {
          onSuccess: () => {
            setStep('code');
            setIsSubmitAttempted(false);
          },
          onError: (error) => {
            notify.scenarios.profile.phoneSendCodeError(error);
          },
        },
      );
    },
    [phone, normalizedPhone, isSamePhone, sendCode],
  );

  const handleVerifyCode = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setIsSubmitAttempted(true);

      if (!isValidCode(code)) {
        return;
      }

      changePhone.mutate(
        { phone: normalizedPhone, code },
        {
          onSuccess: () => {
            notify.scenarios.profile.phoneChanged();
            router.push(ROUTES.profileAccount);
          },
          onError: (error) => {
            notify.scenarios.profile.phoneChangeError(error);
          },
        },
      );
    },
    [code, normalizedPhone, changePhone, router],
  );

  if (step === 'phone') {
    return (
      <form onSubmit={handleSendCode} className={styles.form}>
        <div className={styles.fields}>
          <TextField
            label="Новый номер телефона"
            type="tel"
            inputMode="tel"
            placeholder="+7 (999) 000-00-00"
            value={phone}
            onChange={handlePhoneChange}
            error={phoneError || (isSamePhone ? 'Новый номер совпадает с текущим' : undefined)}
            fullWidth
            autoFocus
          />
        </div>

        <div className={styles.actions}>
          <Button
            type="submit"
            variant="primary"
            size="large"
            fullWidth
            loading={sendCode.isPending}
            disabled={!isValidPhone(phone) || isSamePhone}
          >
            Получить код
          </Button>
        </div>
      </form>
    );
  }

  return (
    <form onSubmit={handleVerifyCode} className={styles.form}>
      <div className={styles.fields}>
        <TextField
          label="Код из SMS"
          type="text"
          inputMode="numeric"
          placeholder="000000"
          value={code}
          onChange={handleCodeChange}
          error={codeError}
          maxLength={CODE_LENGTH}
          fullWidth
          autoFocus
        />
      </div>

      <div className={styles.hint}>
        Код отправлен на {phone}
        <button
          type="button"
          className={styles.changePhoneLink}
          onClick={() => {
            setStep('phone');
            setCode('');
            setIsSubmitAttempted(false);
            sendCode.reset();
            changePhone.reset();
          }}
        >
          Изменить номер
        </button>
      </div>

      <div className={styles.actions}>
        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={changePhone.isPending}
          disabled={!isValidCode(code)}
        >
          Подтвердить
        </Button>
      </div>
    </form>
  );
}

export function PhoneChangeForm(): JSX.Element {
  const { data: me, isPending: isMeLoading, isError: isMeError, refetch } = useMe();

  return (
    <>
      <PageHeader title="Изменение телефона" backHref={ROUTES.profileAccount} />
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
            <TextField
              label="Новый номер телефона"
              type="tel"
              placeholder="+7 (999) 000-00-00"
              value=""
              disabled
              fullWidth
            />
          </div>
          <div className={styles.actions}>
            <Button type="submit" variant="primary" size="large" fullWidth disabled>
              Получить код
            </Button>
          </div>
        </form>
      )}
      {!isMeError && !isMeLoading && me && <PhoneChangeFormView currentPhone={me.phone} />}
    </>
  );
}
