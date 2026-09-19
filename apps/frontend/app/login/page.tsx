"use client";

import {type JSX} from "react";
import {useRouter} from "next/navigation";
import {notify} from "@/shared/lib/notifications";
import {type ApiError} from "@/shared/api/errors";
import {AuthForm, LoginShell, PhoneStep, deviceTimezone, useSendCode, useVerifyCode} from "@/features/auth";
import {normalizePhone, isPhoneValid} from "@/shared/lib/phone";
import {safeInternalPath} from "@/shared/lib/safe-internal-path";
import {useSendCooldown} from "@/features/auth";
import {useLoginDraft} from "@/features/auth";
import {RESEND_TIMEOUT} from "@/features/auth";
import {useStandalone} from "@/shared/lib/hooks/useStandalone";
import {goBack} from "@/shared/lib/navigation";
import styles from "./LoginPage.module.css";

export default function LoginPage(): JSX.Element {
    const router = useRouter();
    const isStandalone = useStandalone();
    const {draft, setDraft, clearDraft} = useLoginDraft();
    const {remainingSeconds: resendTimer, recordSendWithRemainingSeconds} = useSendCooldown();

    const sendCode = useSendCode();
    const verifyCode = useVerifyCode();

    const handleSendCodeError = (error: ApiError) => {
        if (error.status === 429 && typeof error.retryAfter === 'number') {
            recordSendWithRemainingSeconds(error.retryAfter);
        }
        notify.scenarios.auth.loginError({description: error.detail});
    };

    const handleSendPhone = (formattedPhone: string) => {
        if (!isPhoneValid(formattedPhone)) {
            return;
        }

        sendCode.mutate(
            {phone: normalizePhone(formattedPhone)},
            {
                onSuccess: (data) => {
                    if (data.sent) {
                        recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                        setDraft((prev) => ({...prev, phone: formattedPhone, email: "", step: "code"}));
                    } else {
                        setDraft((prev) => ({...prev, phone: formattedPhone, step: "email"}));
                    }
                },
                onError: handleSendCodeError,
            },
        );
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
                    if (!data.sent) {
                        setDraft((prev) => ({...prev, step: "email"}));
                        return;
                    }
                    recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                    setDraft((prev) => ({...prev, step: "code"}));
                },
                onError: handleSendCodeError,
            },
        );
    };

    const handleVerifyCode = (code: string) => {
        const trimmedEmail = draft.email.trim();
        if (code.length !== 6 || !isPhoneValid(draft.phone)) {
            return;
        }

        // Автодетект зоны при регистрации (#451): зона устройства едет с каждой
        // верификацией, бэк применяет её только при создании аккаунта. Когда
        // браузер не отдал зону — поле не отправляется.
        const timezone = deviceTimezone();
        const base = {phone: normalizePhone(draft.phone), code, ...(timezone && {timezone})};

        verifyCode.mutate(
            trimmedEmail ? {...base, email: trimmedEmail} : base,
            {
                onSuccess: () => {
                    const target = safeInternalPath(new URLSearchParams(window.location.search).get("from")) ?? "/properties";
                    router.push(target);
                    clearDraft();
                },
                onError: (error) => {
                    notify.scenarios.auth.loginError({description: error.detail});
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
        goBack(router, "/");
    };

    const handleResend = () => {
        const trimmedEmail = draft.email.trim();
        if (!isPhoneValid(draft.phone)) {
            return;
        }

        sendCode.mutate(
            trimmedEmail
                ? {phone: normalizePhone(draft.phone), email: trimmedEmail}
                : {phone: normalizePhone(draft.phone)},
            {
                onSuccess: (data) => {
                    if (!data.sent) {
                        setDraft((prev) => ({...prev, step: "email"}));
                        return;
                    }
                    recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                },
                onError: handleSendCodeError,
            },
        );
    };

    // Шаг телефона — новый редизайн по макетам Рентли (карта #761, тикет
    // #763). Шаги почты и кода до своих тикетов (#764/#765) живут в прежней
    // вёрстке — ноль регрессий, редизайн следующим экраном.
    if (draft.step === "phone") {
        return (
            <LoginShell onClose={handleClose} hideClose={isStandalone}>
                <PhoneStep
                    phone={draft.phone}
                    onPhoneChange={(phone) => setDraft((prev) => ({...prev, phone}))}
                    onSendPhone={handleSendPhone}
                    isLoading={sendCode.isPending}
                    resendTimer={resendTimer}
                />
            </LoginShell>
        );
    }

    return (
        <main className={styles.container}>
            <section className={styles.left} aria-hidden="true">
                <div className={styles.logo}/>
            </section>
            <section className={styles.right}>
                <div className={styles.formWrapper}>
                    <AuthForm
                        step={draft.step}
                        onStepChange={(step) => setDraft((prev) => ({...prev, step}))}
                        onSendEmail={handleSendEmail}
                        onVerifyCode={handleVerifyCode}
                        onChangePhone={handleChangePhone}
                        onChangeEmail={handleChangeEmail}
                        onResend={handleResend}
                        onClose={handleClose}
                        hideClose={isStandalone}
                        email={draft.email}
                        onEmailChange={(email) => setDraft((prev) => ({...prev, email}))}
                        isSendingEmail={sendCode.isPending}
                        isVerifying={verifyCode.isPending}
                        isResending={sendCode.isPending}
                        resendTimer={resendTimer}
                    />
                </div>
            </section>
            <div className={styles.bgLogo} aria-hidden="true"/>
        </main>
    );
}
