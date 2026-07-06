"use client";

import {type JSX, useState} from "react";
import {useRouter} from "next/navigation";
import {notify} from "@/shared/lib/toast";
import {AuthForm} from "@/features/auth/ui/auth-form";
import {useSendEmailCode, useVerifyEmailCode,} from "@/features/auth/api/hooks";
import {normalizePhone, isPhoneValid} from "@/shared/lib/phone";
import {useSendCooldown} from "@/features/auth/lib/use-send-cooldown";
import {RESEND_TIMEOUT} from "@/features/auth/lib/constants";
import styles from "./LoginPage.module.css";

type Step = "phone" | "email" | "code";

export default function LoginPage(): JSX.Element {
    const router = useRouter();
    const [step, setStep] = useState<Step>("phone");
    const [phone, setPhone] = useState("");
    const [email, setEmail] = useState("");
    const {remainingSeconds: resendTimer, recordSendWithRemainingSeconds} = useSendCooldown();

    const sendEmailCode = useSendEmailCode();
    const verifyEmailCode = useVerifyEmailCode();

    const handleSendPhone = (formattedPhone: string) => {
        if (!isPhoneValid(formattedPhone)) {
            return;
        }

        setPhone(formattedPhone);
        setStep("email");
    };

    const handleSendEmail = () => {
        if (!email || !isPhoneValid(phone)) {
            return;
        }

        sendEmailCode.mutate(
            {phone: normalizePhone(phone), email},
            {
                onSuccess: (data) => {
                    recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                    setStep("code");
                },
                onError: (error) => {
                    if (error.status === 429 && typeof error.retryAfter === 'number') {
                        recordSendWithRemainingSeconds(error.retryAfter);
                    }
                    notify.error(error);
                },
            },
        );
    };

    const handleVerifyCode = (code: string) => {
        if (code.length !== 6 || !isPhoneValid(phone) || !email) {
            return;
        }

        verifyEmailCode.mutate(
            {phone: normalizePhone(phone), email, code},
            {
                onSuccess: () => {
                    router.push("/dashboard");
                },
                onError: (error) => {
                    notify.error(error);
                },
            },
        );
    };

    const handleChangePhone = () => {
        setStep("phone");
    };

    const handleChangeEmail = () => {
        setStep("email");
    };

    const handleClose = () => {
        router.push("/");
    };

    const handleResend = () => {
        if (!isPhoneValid(phone) || !email) {
            return;
        }

        sendEmailCode.mutate(
            {phone: normalizePhone(phone), email},
            {
                onSuccess: (data) => {
                    recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                },
                onError: (error) => {
                    if (error.status === 429 && typeof error.retryAfter === 'number') {
                        recordSendWithRemainingSeconds(error.retryAfter);
                    }
                    notify.error(error);
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
                        onSendEmail={handleSendEmail}
                        onVerifyCode={handleVerifyCode}
                        onChangePhone={handleChangePhone}
                        onChangeEmail={handleChangeEmail}
                        onResend={handleResend}
                        onClose={handleClose}
                        email={email}
                        onEmailChange={setEmail}
                        isSendingEmail={sendEmailCode.isPending}
                        isVerifying={verifyEmailCode.isPending}
                        isResending={sendEmailCode.isPending}
                        resendTimer={resendTimer}
                    />
                </div>
            </section>
            <div className={styles.bgLogo} aria-hidden="true"/>
        </main>
    );
}
