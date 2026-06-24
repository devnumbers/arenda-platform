'use client';

import {type ReactNode, useCallback, useEffect, useState} from 'react';
import clsx from 'clsx';
import {formatPhoneInput} from '@/shared/lib/phone';
import {ArrowLeft, Cancel, Support} from '@/shared/assets/icons';
import {IconButton} from '@/shared/ui/icon-button';
import {PhoneStep} from '../phone-step/PhoneStep';
import {CodeStep} from '../code-step/CodeStep';
import styles from './AuthForm.module.css';

type AuthStep = 'phone' | 'code';

export type AuthFormProps = {
    initialStep?: AuthStep;
    step?: AuthStep;
    onStepChange?: (step: AuthStep) => void;
    onSendPhone?: (phone: string) => void;
    onVerifyCode?: (code: string) => void;
    onChangePhone?: () => void;
    onResend?: () => void;
    onClose?: () => void;
    isSending?: boolean;
    isVerifying?: boolean;
    isResending?: boolean;
    resendTimer?: number;
};

function StepTransition({
                            children,
                            stepKey,
                        }: {
    children: ReactNode;
    stepKey: string;
}) {
    const [isActive, setIsActive] = useState(false);

    useEffect(() => {
        const frame = requestAnimationFrame(() => setIsActive(true));
        return () => cancelAnimationFrame(frame);
    }, [stepKey]);

    return (
        <div
            key={stepKey}
            className={clsx('stepEnter', isActive && 'stepEnterActive', styles.step)}
        >
            {children}
        </div>
    );
}

export function AuthForm({
                             initialStep = 'phone',
                             step: controlledStep,
                             onStepChange,
                             onSendPhone,
                             onVerifyCode,
                             onChangePhone,
                             onResend,
                             onClose,
                             isSending = false,
                             isVerifying = false,
                             isResending = false,
                             resendTimer = 0,
                         }: AuthFormProps) {
    const [internalStep, setInternalStep] = useState<AuthStep>(initialStep);
    const [phone, setPhone] = useState('');
    const [code, setCode] = useState('');

    const effectiveStep = controlledStep ?? internalStep;

    const setStep = useCallback(
        (next: AuthStep) => {
            setInternalStep(next);
            onStepChange?.(next);
        },
        [onStepChange]
    );

    const handlePhoneChange = useCallback((value: string) => {
        setPhone(formatPhoneInput(value));
    }, []);

    const handleCodeChange = useCallback((value: string) => {
        setCode(value);
    }, []);

    const handleSendPhone = useCallback(() => {
        onSendPhone?.(phone);
    }, [onSendPhone, phone]);

    const handleVerifyCode = useCallback(
        (value: string) => {
            onVerifyCode?.(value);
        },
        [onVerifyCode]
    );

    const handleChangePhone = useCallback(() => {
        setStep('phone');
        onChangePhone?.();
    }, [onChangePhone, setStep]);

    const handleResend = useCallback(() => {
        onResend?.();
    }, [onResend]);

    return (
        <div className={styles.root}>
            {effectiveStep === 'phone' ? (
                <div className={styles.topBar}>
                    <IconButton
                        variant="primary-icon"
                        size="medium"
                        aria-label="Закрыть"
                        icon={<Cancel />}
                        onClick={onClose}
                        className={styles.iconButton}
                    />
                </div>
            ) : (
                <div className={clsx(styles.topBar, styles.topBarCode)}>
                    <IconButton
                        variant="primary-icon"
                        size="medium"
                        aria-label="Назад"
                        icon={<ArrowLeft />}
                        onClick={handleChangePhone}
                        className={styles.iconButton}
                    />
                    <IconButton
                        variant="primary-icon"
                        size="medium"
                        aria-label="Написать в поддержку"
                        icon={<Support />}
                        onClick={() => {
                            window.location.href = 'mailto:support@hatus.ru';
                        }}
                        className={styles.iconButton}
                    />
                </div>
            )}
            {effectiveStep === 'phone' ? (
                <StepTransition stepKey="phone">
                    <PhoneStep
                        phone={phone}
                        onPhoneChange={handlePhoneChange}
                        onSubmit={handleSendPhone}
                        isLoading={isSending}
                        resendTimer={resendTimer}
                    />
                </StepTransition>
            ) : (
                <StepTransition stepKey="code">
                    <CodeStep
                        phone={phone}
                        code={code}
                        onCodeChange={handleCodeChange}
                        onVerify={handleVerifyCode}
                        onChangePhone={handleChangePhone}
                        onResend={handleResend}
                        isVerifying={isVerifying}
                        isResending={isResending}
                        resendTimer={resendTimer}
                    />
                </StepTransition>
            )}
        </div>
    );
}
