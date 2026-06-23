'use client';

import {
  useEffect,
  useRef,
  type ChangeEvent,
  type JSX,
} from 'react';
import { ArrowLeft, Clock, Logo } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import styles from './CodeStep.module.css';

export type CodeStepProps = {
  phone: string;
  code: string;
  onCodeChange: (value: string) => void;
  onVerify: (code: string) => void;
  onChangePhone: () => void;
  onResend: () => void;
  isVerifying: boolean;
  isResending: boolean;
  resendTimer: number;
};

export function CodeStep({
  phone,
  code,
  onCodeChange,
  onVerify,
  onChangePhone,
  onResend,
  isVerifying,
  isResending,
  resendTimer,
}: CodeStepProps): JSX.Element {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const digits = event.target.value.replace(/\D/g, '').slice(0, 6);
    onCodeChange(digits);
    if (digits.length === 6) {
      onVerify(digits);
    }
  };

  const minutes = String(Math.floor(resendTimer / 60)).padStart(2, '0');
  const seconds = String(resendTimer % 60).padStart(2, '0');
  const timerText = `${minutes}:${seconds}`;

  return (
    <div className={styles.root}>
      <Logo className={styles.logo} />
      <div className={styles.header}>
        <h1 className={styles.title}>Введите код</h1>
        <p className={styles.subtitle}>
          Отправили СМС-код на номер {phone}
        </p>
      </div>

      <div className={styles.fields}>
        <TextField
          ref={inputRef}
          labelPlacement="inside"
          label="6-значный код"
          maxLength={6}
          inputMode="numeric"
          value={code}
          onChange={handleChange}
          fullWidth
        />
      </div>

      <div className={styles.buttons}>
        <Button
          variant="primary"
          size="large"
          fullWidth
          disabled={resendTimer > 0 || isResending}
          loading={isResending}
          leftIcon={<Clock />}
          onClick={onResend}
        >
          <span className={styles.resendContent}>
            <span>Отправить новый код</span>
            {resendTimer > 0 && (
              <span className={styles.resendTimer}>{timerText}</span>
            )}
          </span>
        </Button>

        <Button
          variant="clear"
          size="large"
          fullWidth
          leftIcon={<ArrowLeft />}
          onClick={onChangePhone}
          disabled={isVerifying}
        >
          Изменить номер
        </Button>
      </div>
    </div>
  );
}
