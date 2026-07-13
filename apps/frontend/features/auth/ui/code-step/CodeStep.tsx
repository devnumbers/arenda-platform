'use client';

import {type ChangeEvent, type JSX, useEffect, useRef} from 'react';
import {ArrowLeft} from '@/shared/assets/icons';
import {SendCodeButton} from '@/features/auth/ui/send-code-button';
import {Button} from '@/shared/ui/button';
import {TextField} from '@/shared/ui/text-field';
import styles from './CodeStep.module.css';

export type CodeStepProps = {
    contact: string;
    contactType: 'email' | 'stored-email';
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
        ? 'Отправили код на почту'
        : 'Мы отправили код на вашу почту';

    const changeLabel = contactType === 'email' ? 'Изменить почту' : 'Изменить номер';

    return (
        <div className={styles.root}>
            <div className={styles.header}>
                <h1 className={styles.title}>Введите код</h1>
                <p className={styles.subtitle}>
                    {contactType === 'email' ? (
                        <>
                            {contactLabel}{' '}
                            <span className={styles.contact}>{contact}</span>
                        </>
                    ) : (
                        contactLabel
                    )}
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
                <SendCodeButton
                    remainingSeconds={resendTimer}
                    loading={isResending}
                    disabled={isResending}
                    onClick={onResend}
                >
                    Отправить новый код
                </SendCodeButton>

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

