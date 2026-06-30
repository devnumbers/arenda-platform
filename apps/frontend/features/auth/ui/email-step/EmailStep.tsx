'use client';

import { type ChangeEvent, type JSX, useState } from 'react';
import { Button } from '@/shared/ui/button';
import { TextField } from '@/shared/ui/text-field';
import styles from './EmailStep.module.css';

const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export type EmailStepProps = {
  email: string;
  onEmailChange: (value: string) => void;
  onSubmit: () => void;
  isLoading: boolean;
};

export function EmailStep({
  email,
  onEmailChange,
  onSubmit,
  isLoading,
}: EmailStepProps): JSX.Element {
  const [touched, setTouched] = useState(false);
  const trimmed = email.trim();
  const isValid = emailRegex.test(trimmed);
  const showError = touched && !isValid && trimmed !== '';

  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onEmailChange(event.target.value);
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setTouched(true);
    if (isValid) {
      onSubmit();
    }
  };

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <h1 className={styles.title}>Введите email</h1>
        <p className={styles.subtitle}>На него придёт код для входа</p>
      </div>

      <form className={styles.form} onSubmit={handleSubmit}>
        <TextField
          labelPlacement="inside"
          label="Email"
          type="email"
          autoComplete="email"
          placeholder="you@example.com"
          value={email}
          onChange={handleChange}
          error={showError ? 'Введите корректный email' : undefined}
          fullWidth
        />

        <Button
          type="submit"
          variant="primary"
          size="large"
          fullWidth
          loading={isLoading}
          disabled={!isValid || isLoading}
        >
          Получить код
        </Button>
      </form>
    </div>
  );
}
