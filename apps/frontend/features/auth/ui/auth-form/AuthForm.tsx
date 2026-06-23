'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import clsx from 'clsx';
import { formatPhoneInput } from '@/shared/lib/phone';
import { PhoneStep } from '../phone-step/PhoneStep';
import { CodeStep } from '../code-step/CodeStep';
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
        <StepTransition stepKey="phone">
          <PhoneStep
            phone={phone}
            onPhoneChange={handlePhoneChange}
            onSubmit={handleSendPhone}
            isLoading={isSending}
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
