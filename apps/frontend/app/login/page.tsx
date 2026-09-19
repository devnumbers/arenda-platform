"use client";

import {type JSX, useState} from "react";
import {useRouter} from "next/navigation";
import {notify} from "@/shared/lib/notifications";
import {type ApiError} from "@/shared/api/errors";
import {CodeStep, EmailStep, LoginShell, PhoneStep, deviceTimezone, useSendCode, useVerifyCode} from "@/features/auth";
import {isEmailValid} from "@/shared/lib/email";
import {normalizePhone, isPhoneValid} from "@/shared/lib/phone";
import {safeInternalPath} from "@/shared/lib/safe-internal-path";
import {useSendCooldown} from "@/features/auth";
import {useLoginDraft} from "@/features/auth";
import {RESEND_TIMEOUT} from "@/features/auth";
import {useStandalone} from "@/shared/lib/hooks/useStandalone";
import {goBack} from "@/shared/lib/navigation";

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
        if (!isEmailValid(trimmedEmail) || !isPhoneValid(draft.phone)) {
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

    const [code, setCode] = useState("");
    const [verifyError, setVerifyError] = useState<string | null>(null);

    const handleCodeChange = (value: string) => {
        setCode(value);
        setVerifyError(null);
    };

    const handleVerifyCode = (value: string) => {
        const trimmedEmail = draft.email.trim();
        if (value.length !== 6 || !isPhoneValid(draft.phone) || verifyCode.isPending) {
            return;
        }

        // Автодетект зоны при регистрации (#451): зона устройства едет с каждой
        // верификацией, бэк применяет её только при создании аккаунта. Когда
        // браузер не отдал зону — поле не отправляется.
        const timezone = deviceTimezone();
        const base = {phone: normalizePhone(draft.phone), code: value, ...(timezone && {timezone})};

        verifyCode.mutate(
            trimmedEmail ? {...base, email: trimmedEmail} : base,
            {
                onSuccess: () => {
                    const target = safeInternalPath(new URLSearchParams(window.location.search).get("from")) ?? "/properties";
                    router.push(target);
                    clearDraft();
                },
                onError: (error) => {
                    // Неверный код (401) — инлайн в поле по макету 2349:67624
                    // (resend-канон #733: 401 verify — не тост; текст макета
                    // «Неверный код», деталь бэка «Неверный телефон, почта или
                    // код» не встаёт в поле). Блокировка (429 — окно попыток
                    // 15/30 мин, бан, лимит) — понятный текст бэка тоже инлайн,
                    // чтобы не исчезал. Остальное — тост сценария.
                    if (error.status === 401 || error.status === 429) {
                        setVerifyError(error.status === 401 ? "Неверный код" : error.detail);
                        return;
                    }
                    notify.scenarios.auth.loginError({description: error.detail});
                },
            },
        );
    };

    const handleCodeClear = () => {
        setCode("");
        setVerifyError(null);
    };

    // Стрелка ← — на предыдущий шаг: почта, если она была, иначе телефон
    // (макеты #765). Черновик кода и ошибка живут только на шаге кода.
    // Пока верификация в полёте, уход со шага закрыт — иначе её onSuccess
    // утащил бы пользователя в кабинет с чужого шага.
    const handleCodeBack = () => {
        if (verifyCode.isPending) {
            return;
        }
        handleCodeClear();
        setDraft((prev) => ({...prev, step: prev.email.trim() ? "email" : "phone"}));
    };

    const handleResend = () => {
        const trimmedEmail = draft.email.trim();
        if (!isPhoneValid(draft.phone) || verifyCode.isPending) {
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
                    handleCodeClear();
                },
                onError: handleSendCodeError,
            },
        );
    };

    const handleClose = () => {
        goBack(router, "/");
    };

    // Шаги телефона и почты — редизайн по макетам Рентли (карта #761,
    // тикеты #763/#764), шаг кода — #765. Все три шага живут на LoginShell.
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

    if (draft.step === "email") {
        return (
            <LoginShell onClose={handleClose} hideClose={isStandalone}>
                <EmailStep
                    email={draft.email}
                    onEmailChange={(email) => setDraft((prev) => ({...prev, email}))}
                    onSubmit={handleSendEmail}
                    isLoading={sendCode.isPending}
                    resendTimer={resendTimer}
                />
            </LoginShell>
        );
    }

    return (
        <LoginShell onClose={handleClose} hideClose={isStandalone} onBack={handleCodeBack}>
            <CodeStep
                code={code}
                onCodeChange={handleCodeChange}
                onVerify={handleVerifyCode}
                error={verifyError ?? undefined}
                onClear={handleCodeClear}
                onResend={handleResend}
                isResending={sendCode.isPending}
                resendTimer={resendTimer}
            />
        </LoginShell>
    );
}
