'use client';

import {type JSX, useState} from 'react';
import {useRouter} from 'next/navigation';
import {toast} from 'react-toastify';
import {AuthForm} from '@/features/auth/ui/auth-form';
import {useSendPhoneCode, useVerifyPhoneCode} from '@/features/auth/api/hooks';
import {normalizePhone} from '@/features/auth/lib/normalize-phone';
import {useSendCooldown} from '@/features/auth/lib/use-send-cooldown';
import styles from './LoginPage.module.css';

type Step = 'phone' | 'code';

export default function LoginPage(): JSX.Element {
    const router = useRouter();
    const [step, setStep] = useState<Step>('phone');
    const [phone, setPhone] = useState('');
    const {remainingSeconds: resendTimer, recordSend} = useSendCooldown();

    const sendPhoneCode = useSendPhoneCode();
    const verifyPhoneCode = useVerifyPhoneCode();

    const handleSendPhone = (formattedPhone: string) => {
        if (formattedPhone.length < 18) {
            return;
        }

        setPhone(formattedPhone);

        sendPhoneCode.mutate(
            {phone: normalizePhone(formattedPhone)},
            {
                onSuccess: () => {
                    recordSend();
                    setStep('code');
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
            {phone: normalizePhone(phone), code},
            {
                onSuccess: () => {
                    router.push('/dashboard');
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

    const handleClose = () => {
        router.push('/');
    };

    const handleResend = () => {
        if (phone.length < 18) {
            return;
        }

        sendPhoneCode.mutate(
            {phone: normalizePhone(phone)},
            {
                onSuccess: () => {
                    recordSend();
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
                <div className={styles.logo}/>
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
                        onClose={handleClose}
                        isSending={sendPhoneCode.isPending}
                        isVerifying={verifyPhoneCode.isPending}
                        isResending={sendPhoneCode.isPending}
                        resendTimer={resendTimer}
                    />
                </div>
            </section>
            <div className={styles.bgLogo} aria-hidden="true"/>
        </main>
    );
}
