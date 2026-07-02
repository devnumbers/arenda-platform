'use client';

import {type ChangeEvent, type JSX, useEffect, useRef,} from 'react';
import {ArrowLeft, Clock} from '@/shared/assets/icons';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import {formatTimer} from '@/features/auth/lib/format-timer';
import styles from './CodeStep.module.css';

export type CodeStepProps = {
    contact: string;
    contactType: 'phone' | 'email';
    code: string;
    onCodeChange: (value: string) => void;
    onVerify: (code: string) => void;
    onChangeContact: () => void;
    onResend: () => void;
    isVerifying: boolean;
    isResending: boolean;
    resendTimer: number;
};

export function CodeStep({
                             contact,
                             contactType,
                             code,
                             onCodeChange,
                             onVerify,
                             onChangeContact,
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

    const contactLabel = contactType === 'email'
        ? 'Отправили код на email'
        : 'Отправили СМС-код на номер';

    const changeLabel = contactType === 'email' ? 'Изменить email' : 'Изменить номер';

    return (
        <div className={styles.root}>
            <div className={styles.header}>
                <h1 className={styles.title}>Введите код</h1>
                <p className={styles.subtitle}>
                    {contactLabel}{' '}
                    <span className={styles.contact}>{contact}</span>
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
                    onClick={onResend}
                    subtitle={
                        resendTimer > 0 ? (
                            <span className={styles.timerRow}>
                <Clock className={styles.timerIcon}/>
                <span className={styles.timerText}>{formatTimer(resendTimer)}</span>
              </span>
                        ) : undefined
                    }
                >
                    Отправить новый код
                </Button>

                <Button
                    variant="clear"
                    size="large"
                    fullWidth
                    leftIcon={<ArrowLeft/>}
                    onClick={onChangeContact}
                    disabled={isVerifying}
                >
                    {changeLabel}
                </Button>
            </div>
        </div>
    );
}

