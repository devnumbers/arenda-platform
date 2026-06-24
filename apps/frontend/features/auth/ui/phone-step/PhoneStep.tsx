'use client';

import { type ChangeEvent, type JSX } from 'react';
import { formatPhoneInput } from '@/shared/lib/phone';
import { Clock, Logo, Support } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import { formatTimer } from '@/features/auth/lib/format-timer';
import styles from './PhoneStep.module.css';

export type PhoneStepProps = {
  phone: string;
  onPhoneChange: (value: string) => void;
  onSubmit: () => void;
  isLoading: boolean;
  resendTimer?: number;
};

export function PhoneStep({
  phone,
  onPhoneChange,
  onSubmit,
  isLoading,
  resendTimer = 0,
}: PhoneStepProps): JSX.Element {
  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onPhoneChange(formatPhoneInput(event.target.value));
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit();
  };

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <h1 className={styles.title}>Введите номер телефона</h1>
        <p className={styles.subtitle}>Чтобы войти или зарегистрироваться</p>
      </div>

      <form className={styles.form} onSubmit={handleSubmit}>
        <TextField
          labelPlacement="inside"
          label="Телефон"
          placeholder="+7 (000) 000-00-00"
          value={phone}
          onChange={handleChange}
          fullWidth
        />

        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={isLoading}
          disabled={phone.length < 18 || isLoading || resendTimer > 0}
          subtitle={
            resendTimer > 0 ? (
              <span className={styles.timerRow}>
                <Clock className={styles.timerIcon} />
                <span className={styles.timerText}>{formatTimer(resendTimer)}</span>
              </span>
            ) : undefined
          }
        >
          {resendTimer > 0 ? 'Отправить новый код' : 'Войти'}
        </Button>
      </form>

      <p className={styles.legal}>
        Нажимая кнопку «Войти», я принимаю{' '}
        <a className={styles.legalLink} href="#">
          политику конфиденциальности
        </a>{' '}
        и{' '}
        <a className={styles.legalLink} href="#">
          соглашаюсь на обработку персональных данных
        </a>
      </p>

      <Button
        variant="clear"
        size="large"
        fullWidth
        leftIcon={<Support />}
        onClick={() => {
          window.location.href = 'mailto:support@hatus.ru';
        }}
      >
        Написать в поддержку
      </Button>
    </div>
  );
}

