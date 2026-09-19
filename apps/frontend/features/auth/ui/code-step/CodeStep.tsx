'use client';

import { type ChangeEvent, type JSX, useEffect, useRef } from "react";
import { isValidLoginCode, loginCodeFromInput } from "@/shared/lib/login-code";
import { HeaderLogo, ResendCodeTile, TextField } from "@/shared/ui/design";

export type CodeStepProps = {
    code: string;
    onCodeChange: (value: string) => void;
    onVerify: (code: string) => void;
    /** Инлайн-ошибка поля: «Неверный код» (401 verify) или понятный текст
     * блокировки (429). Пусто — поле в покое. */
    error?: string;
    onClear: () => void;
    onResend: () => void;
    isResending: boolean;
    resendTimer?: number;
};

/** Шаг «Введите код» по макетам Рентли (карта #761, тикет #765; Figma
 * 2349:67383 — мобайл, 2349:67396 — планшет, 2349:67411 — десктоп,
 * 2349:67624 — ошибка). Колонка блоков с зазором 32, как на шагах
 * телефона #763 и почты #764: лого — канон HeaderLogo 112×28; заголовок
 * H1 28/32 + подпись 16/18 «Отправили 6-значный код на вашу почту»
 * (один текст для обоих флоу — код по ADR 0044 всегда уходит письмом,
 * макет контакт не показывает). Поле — канон TextField titleIn «Код»:
 * маска 6 цифр с автосабмитом (как в прежнем шаге кода), крестик очистки;
 * нативный maxLength не ставится — он рисует счётчик канона (в макете
 * его нет) и обрезает вставку до маски, теряя цифры хвоста; потолок
 * держит сама маска. Ошибка верификации — инлайн в error-проп
 * (resend-канон #733: 401 — не тост), по макету поле краснеет, крестик
 * становится красным.
 * Отправка нового кода — канон ResendCodeTile (#733): Secondary-кнопка
 * «Отправить новый код», в кулдауне 60 с от retryAfter погашена с
 * подписью «Запросить новый код можно через ММ:СС» (моно 14/16). Зазор
 * поле — плитка 24 по канону #733. Стрелка ← — в баре шейла (onBack
 * LoginShell), не в колонке контента. */
export function CodeStep({
    code,
    onCodeChange,
    onVerify,
    error,
    onClear,
    onResend,
    isResending,
    resendTimer = 0,
}: CodeStepProps): JSX.Element {
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        const digits = loginCodeFromInput(event.target.value);
        onCodeChange(digits);
        if (isValidLoginCode(digits)) {
            onVerify(digits);
        }
    };

    return (
        <div className="flex w-full flex-col gap-8">
            <HeaderLogo className="h-7 w-28" />

            <div className="flex flex-col gap-2">
                <h1 className="m-0 text-[28px] font-semibold leading-8 text-content">
                    Введите код
                </h1>
                <p className="m-0 text-base leading-[18px] text-content-secondary">
                    Отправили 6-значный код на вашу почту
                </p>
            </div>

            <div className="flex flex-col gap-6">
                <TextField
                    ref={inputRef}
                    variant="titleIn"
                    title="Код"
                    name="code"
                    type="text"
                    inputMode="numeric"
                    value={code}
                    onChange={handleChange}
                    onClear={onClear}
                    error={error}
                />

                <ResendCodeTile
                    remainingSeconds={resendTimer}
                    loading={isResending}
                    onResend={onResend}
                />
            </div>
        </div>
    );
}
