"use client";

import {type JSX, useRef, useState} from "react";
import {notify} from "@/shared/lib/notifications";
import {type ApiError} from "@/shared/api/errors";
import {CodeStep, EmailStep, LoginShell, deviceTimezone, useSendCode, useVerifyCode} from "@/features/auth";
import {isEmailValid} from "@/shared/lib/email";
import {safeInternalPath} from "@/shared/lib/safe-internal-path";
import {useSendCooldown} from "@/features/auth";
import {useLoginDraft} from "@/features/auth";
import {RESEND_TIMEOUT} from "@/features/auth";
import {useStandalone} from "@/shared/lib/hooks/useStandalone";
import {hardReplace} from "@/shared/lib/navigation";

export default function LoginPage(): JSX.Element {
    const isStandalone = useStandalone();
    const {draft, setDraft, clearDraft} = useLoginDraft();
    const {remainingSeconds: resendTimer, recordSendWithRemainingSeconds} = useSendCooldown();

    const sendCode = useSendCode();
    const verifyCode = useVerifyCode();

    // Двойной клик по кнопке отправки (#1099): кнопка диспейблится isPending-ем
    // только после перерисовки, а второй клик может прийти раньше рендера
    // (джанк главного потока отдаёт input-событие раньше рендер-таска) — без
    // синхронного рефа обе отправки уходят POST-ами, и вторая ловит 429
    // троттлинга бэка (minSendInterval 60с) тостом поверх ушедшего шага.
    const sendInFlightRef = useRef(false);

    // Единственное горло всех отправок кода (#1099): реф режет второй клик из
    // окна до перерисовки, isPending догашивает окно между settled и самой
    // перерисовкой; жизненный цикл рефа (set до mutate, сброс в onSettled)
    // живёт здесь в одном месте. Один реф на оба обработчика: шаг почты и
    // resend не пересекаются — это разные шаги черновика.
    const startSendOnce = (...[variables, callbacks]: Parameters<typeof sendCode.mutate>): void => {
        if (sendInFlightRef.current || sendCode.isPending) {
            return;
        }

        sendInFlightRef.current = true;
        sendCode.mutate(variables, {
            ...callbacks,
            onSettled: () => {
                sendInFlightRef.current = false;
            },
        });
    };

    const handleSendCodeError = (error: ApiError) => {
        if (error.status === 429 && typeof error.retryAfter === 'number') {
            recordSendWithRemainingSeconds(error.retryAfter);
        }
        notify.scenarios.auth.loginError({description: error.detail});
    };

    const handleSendEmail = () => {
        const trimmedEmail = draft.email.trim();
        if (!isEmailValid(trimmedEmail)) {
            return;
        }

        // Ответ не раскрывает существование аккаунта (анти-enumeration,
        // research #1526): бэк всегда отвечает sent:true, вход = регистрация
        // (ADR 0067) — единственный путь ведёт на шаг кода.
        startSendOnce({email: trimmedEmail}, {
            onSuccess: (data) => {
                recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                setDraft((prev) => ({...prev, step: "code"}));
            },
            onError: handleSendCodeError,
        });
    };

    const [code, setCode] = useState("");
    const [verifyError, setVerifyError] = useState<string | null>(null);

    const handleCodeChange = (value: string) => {
        setCode(value);
        setVerifyError(null);
    };

    const handleVerifyCode = (value: string) => {
        const trimmedEmail = draft.email.trim();
        if (value.length !== 6 || !isEmailValid(trimmedEmail) || verifyCode.isPending) {
            return;
        }

        // Автодетект зоны при регистрации (#451): зона устройства едет с каждой
        // верификацией, бэк применяет её только при создании аккаунта. Когда
        // браузер не отдал зону — поле не отправляется.
        const timezone = deviceTimezone();

        verifyCode.mutate(
            {email: trimmedEmail, code: value, ...(timezone && {timezone})},
            {
                onSuccess: () => {
                    const target = safeInternalPath(new URLSearchParams(window.location.search).get("from")) ?? "/properties";
                    clearDraft();
                    // Жёсткая навигация auth-границы (#1098, канон Navigation):
                    // сносит гостевые RSC-остатки и react-query предыдущего
                    // документа вместо перевозки их в кабинет.
                    hardReplace(target);
                },
                onError: (error) => {
                    // Неверный код (401) — инлайн в поле по макету 2349:67624
                    // (resend-канон #733: 401 verify — не тост; текст макета
                    // «Неверный код», деталь бэка «Неверная почта или код» не
                    // встаёт в поле). Блокировка (429 — окно попыток 15/30 мин,
                    // бан, лимит) — понятный текст бэка тоже инлайн, чтобы не
                    // исчезал. Остальное — тост сценария.
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

    // Стрелка ← код-шага — на шаг почты (макеты #765). Черновик кода и
    // ошибка живут только на шаге кода. Пока верификация в полёте, уход со
    // шага закрыт — иначе её onSuccess утащил бы пользователя в кабинет
    // с чужого шага.
    const handleCodeBack = () => {
        if (verifyCode.isPending) {
            return;
        }
        handleCodeClear();
        setDraft((prev) => ({...prev, step: "email"}));
    };

    const handleResend = () => {
        const trimmedEmail = draft.email.trim();
        if (!isEmailValid(trimmedEmail) || verifyCode.isPending) {
            return;
        }

        startSendOnce({email: trimmedEmail}, {
            onSuccess: (data) => {
                recordSendWithRemainingSeconds(data.retryAfter ?? RESEND_TIMEOUT);
                handleCodeClear();
            },
            onError: handleSendCodeError,
        });
    };

    // Крестик — выход на лендинг (#1103, слово владельца): сайт всегда
    // заменяет адрес на «/» без учёта истории — goBack вернул бы гостя на
    // кабинетный маршрут (например ?from=/properties), где proxy.ts снова
    // редиректит на /login?from…: крестик закрывал бы форму и тут же
    // открывал её заново. В standalone PWA крестика нет вовсе (hideClose).
    // Жёсткая навигация auth-границы — канон Navigation (#1098).
    const handleClose = () => {
        hardReplace("/");
    };

    // Шаг почты — вход в поток (ADR 0067, #1529): вход = регистрация одним
    // потоком «email → код», прежний телефонный шаг снесён, стрелки ← у
    // первого шага нет. Шаг кода — #765. Оба шага живут на LoginShell.
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
