'use client';

import { useEffect, useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'react-toastify';
import { AuthForm } from '@/features/auth/ui/auth-form';
import { useSendPhoneCode, useVerifyPhoneCode } from '@/features/auth/api/hooks';
import { normalizePhone } from '@/features/auth/lib/normalize-phone';
import styles from './LoginPage.module.css';

type Step = 'phone' | 'code';

const RESEND_TIMEOUT = 60;

export default function LoginPage(): JSX.Element {
  const router = useRouter();
  const [step, setStep] = useState<Step>('phone');
  const [phone, setPhone] = useState('');
  const [resendTimer, setResendTimer] = useState(RESEND_TIMEOUT);

  const sendPhoneCode = useSendPhoneCode();
  const verifyPhoneCode = useVerifyPhoneCode();

  useEffect(() => {
    if (step !== 'code' || resendTimer <= 0) {
      return;
    }

    const interval = setInterval(() => {
      setResendTimer((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);

    return () => clearInterval(interval);
  }, [step, resendTimer]);

  const handleSendPhone = (formattedPhone: string) => {
    if (formattedPhone.length < 18) {
      return;
    }

    setPhone(formattedPhone);

    sendPhoneCode.mutate(
      { phone: normalizePhone(formattedPhone) },
      {
        onSuccess: () => {
          setStep('code');
          setResendTimer(RESEND_TIMEOUT);
        },
        onError: (error) => {
          toast.error(error.detail);
        },
      },
    );
  };

  const handleVerifyCode = (code: string) => {
    if (code.length !== 6 || phone.length < 18) {
      return;
    }

    verifyPhoneCode.mutate(
      { phone: normalizePhone(phone), code },
      {
        onSuccess: () => {
          router.push('/');
        },
        onError: () => {
          toast.error('Неверный код');
        },
      },
    );
  };

  const handleChangePhone = () => {
    setStep('phone');
  };

  const handleResend = () => {
    if (phone.length < 18) {
      return;
    }

    sendPhoneCode.mutate(
      { phone: normalizePhone(phone) },
      {
        onSuccess: () => {
          setResendTimer(RESEND_TIMEOUT);
        },
        onError: (error) => {
          toast.error(error.detail);
        },
      },
    );
  };

  return (
    <main className={styles.container}>
      <section className={styles.left} aria-hidden="true">
        <div className={styles.logo} />
      </section>
      <section className={styles.right}>
        <div className={styles.formWrapper}>
          <AuthForm
            step={step}
            onStepChange={setStep}
            onSendPhone={handleSendPhone}
            onVerifyCode={handleVerifyCode}
            onChangePhone={handleChangePhone}
            onResend={handleResend}
            isSending={sendPhoneCode.isPending}
            isVerifying={verifyPhoneCode.isPending}
            isResending={sendPhoneCode.isPending}
            resendTimer={resendTimer}
          />
        </div>
      </section>
    </main>
  );
}
