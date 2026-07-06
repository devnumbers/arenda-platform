"use client";

import {type JSX} from "react";
import {useRouter} from "next/navigation";
import {notify} from "@/shared/lib/toast";
import {AuthForm} from "@/features/auth/ui/auth-form";
import {useSendCode, useVerifyCode} from "@/features/auth/api/hooks";
import {normalizePhone, isPhoneValid} from "@/shared/lib/phone";
import {useSendCooldown} from "@/features/auth/lib/use-send-cooldown";
import {useLoginDraft} from "@/features/auth/lib/use-login-draft";
import {RESEND_TIMEOUT} from "@/features/auth/lib/constants";
import styles from "./LoginPage.module.css";

export default function LoginPage(): JSX.Element {
    const router = useRouter();
    const {draft, setDraft, clearDraft, isLoaded} = useLoginDraft();
    const {remainingSeconds: resendTimer, recordSendWithRemainingSeconds} = useSendCooldown();

    const sendCode = useSendCode();
    const verifyCode = useVerifyCode();

    const handleSendPhone = (formattedPhone: string) => {
        if (!isPhoneValid(formattedPhone)) {
            return;
        }

        setDraft((prev) => ({...prev, phone: formattedPhone, step: "email"}));
    };

    const handleSendEmail = () => {
        const trimmedEmail = draft.email.trim();
        if (!trimmedEmail || !isPhoneValid(draft.phone)) {
            return;
        }

        sendCode.mutate(
            {phone: normalizePhone(draft.phone), email: trimmedEmail},
            {
                onSuccess: (data) => {
                    recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                    setDraft((prev) => ({...prev, step: "code"}));
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
        const trimmedEmail = draft.email.trim();
        if (code.length !== 6 || !isPhoneValid(draft.phone) || !trimmedEmail) {
            return;
        }

        verifyCode.mutate(
            {phone: normalizePhone(draft.phone), email: trimmedEmail, code},
            {
                onSuccess: () => {
                    router.push("/dashboard");
                    clearDraft();
                },
                onError: (error) => {
                    notify.error(error);
                },
            },
        );
    };

    const handleChangePhone = () => {
        setDraft((prev) => ({...prev, step: "phone"}));
    };

    const handleChangeEmail = () => {
        setDraft((prev) => ({...prev, step: "email"}));
    };

    const handleClose = () => {
        router.push("/");
    };

    const handleResend = () => {
        const trimmedEmail = draft.email.trim();
        if (!isPhoneValid(draft.phone) || !trimmedEmail) {
            return;
        }

        sendCode.mutate(
            {phone: normalizePhone(draft.phone), email: trimmedEmail},
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
                    {!isLoaded ? (
                        <div>Загрузка…</div>
                    ) : (
                        <AuthForm
                        step={draft.step}
                        onStepChange={(step) => setDraft((prev) => ({...prev, step}))}
                        onSendPhone={handleSendPhone}
                        onSendEmail={handleSendEmail}
                        onVerifyCode={handleVerifyCode}
                        onChangePhone={handleChangePhone}
                        onChangeEmail={handleChangeEmail}
                        onResend={handleResend}
                        onClose={handleClose}
                        phone={draft.phone}
                        onPhoneChange={(phone) => setDraft((prev) => ({...prev, phone}))}
                        email={draft.email}
                        onEmailChange={(email) => setDraft((prev) => ({...prev, email}))}
                        isSendingEmail={sendCode.isPending}
                        isVerifying={verifyCode.isPending}
                        isResending={sendCode.isPending}
                        resendTimer={resendTimer}
                    />
                    )}
                </div>
            </section>
            <div className={styles.bgLogo} aria-hidden="true"/>
        </main>
    );
}
