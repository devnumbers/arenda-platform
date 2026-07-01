"use client";

import {type JSX, useState} from "react";
import {useRouter} from "next/navigation";
import {toast} from "react-toastify";
import {AuthForm} from "@/features/auth/ui/auth-form";
import {useSendEmailCode, useVerifyEmailCode,} from "@/features/auth/api/hooks";
import {normalizePhone} from "@/features/auth/lib/normalize-phone";
import {useSendCooldown} from "@/features/auth/lib/use-send-cooldown";
import styles from "./LoginPage.module.css";

type Step = "phone" | "email" | "code";

export default function LoginPage(): JSX.Element {
    const router = useRouter();
    const [step, setStep] = useState<Step>("phone");
    const [phone, setPhone] = useState("");
    const [email, setEmail] = useState("");
    const {remainingSeconds: resendTimer, recordSend} = useSendCooldown();

    const sendEmailCode = useSendEmailCode();
    const verifyEmailCode = useVerifyEmailCode();

    const handleSendPhone = (formattedPhone: string) => {
        if (formattedPhone.length < 18) {
            return;
        }

        setPhone(formattedPhone);
        setStep("email");
    };

    const handleSendEmail = () => {
        if (!email || phone.length < 18) {
            return;
        }

        recordSend();
        sendEmailCode.mutate(
            {phone: normalizePhone(phone), email},
            {
                onSuccess: () => {
                    setStep("code");
                },
                onError: (error) => {
                    toast.error(error.detail);
                },
            },
        );
    };

    const handleVerifyCode = (code: string) => {
        if (code.length !== 6 || phone.length < 18 || !email) {
            return;
        }

        verifyEmailCode.mutate(
            {phone: normalizePhone(phone), email, code},
            {
                onSuccess: () => {
                    router.push("/dashboard");
                },
                onError: (error) => {
                    toast.error(error.detail ?? "Неверный код");
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
        if (phone.length < 18 || !email) {
            return;
        }

        recordSend();
        sendEmailCode.mutate(
            {phone: normalizePhone(phone), email},
            {
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
