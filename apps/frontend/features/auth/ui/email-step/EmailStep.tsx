'use client';

import {type ChangeEvent, type JSX, useState} from 'react';
import {SendCodeButton} from '@/features/auth/ui/send-code-button';
import {TextField} from '@/shared/ui/text-field';
import styles from './EmailStep.module.css';

const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export type EmailStepProps = {
    email: string;
    onEmailChange: (value: string) => void;
    onSubmit: () => void;
    isLoading: boolean;
    resendTimer?: number;
};

export function EmailStep({
                              email,
                              onEmailChange,
                              onSubmit,
                              isLoading,
                              resendTimer = 0,
                          }: EmailStepProps): JSX.Element {
    const [touched, setTouched] = useState(false);
    const trimmed = email.trim();
    const isValid = emailRegex.test(trimmed);
    const showError = touched && !isValid;

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

    const errorMessage = trimmed === '' ? 'Введите email' : 'Введите корректный email';

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
                    error={showError ? errorMessage : undefined}
                    fullWidth
                />

                <SendCodeButton
                    type="submit"
                    remainingSeconds={resendTimer}
                    loading={isLoading}
                    disabled={!isValid || isLoading || resendTimer > 0}
                    timerLabel={() => `Отправить новый код`}
                >
                    Получить код
                </SendCodeButton>
            </form>
        </div>
    );
}
