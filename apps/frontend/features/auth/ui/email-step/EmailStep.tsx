'use client';

import { type ChangeEvent, type JSX } from "react";
import { isEmailValid } from "@/shared/lib/email";
import { Button, TextField } from "@/shared/ui/design";
import { CooldownNote, StepHeader, SupportRequestButton } from "../step-chrome";

export type EmailStepProps = {
    email: string;
    onEmailChange: (value: string) => void;
    onSubmit: () => void;
    isLoading: boolean;
    resendTimer?: number;
};

/** Шаг «Введите почту» по макетам Рентли (карта #761, тикет #764; Figma
 * 2349:67698 — мобайл, пустое, 2349:67820 — заполненное; десктоп/планшет
 * в файле не нарисованы — собраны по шейлу телефона #763, согласовать на
 * приёмке). Колонка блоков с зазором 32: лого-вектор 112×28 — канон
 * HeaderLogo; заголовок H1 28/32 + подпись 16/18 (зазор 8); поле — канон
 * TextField titleIn (плавающий лейбл «Электронная почта», крестик очистки
 * при значении) + Primary-кнопка (зазор 16); «Написать в поддержку» —
 * White-пилюля с Icon/R/Support, как на шаге телефона. Юртекста на макете
 * почты нет — consent дан на шаге телефона. Кнопка в покое погашена
 * (disabled до валидной почты; isEmailValid — та же свободная форма
 * «что-то@домен.тлд», что на бэкенде). Кулдаун 429/retryAfter — кнопка
 * погашена + подпись resend-канона (#733) «Запросить код можно через
 * ММ:СС» (моно 14/16, tertiary). Шаг видят только новые пользователи
 * (sent:false из POST /auth/send, ADR 0015) — код уходит на введённую
 * почту. «Написать в поддержку» открывает модалку «Связаться с нами»
 * (#766). */
export function EmailStep({
    email,
    onEmailChange,
    onSubmit,
    isLoading,
    resendTimer = 0,
}: EmailStepProps): JSX.Element {
    const isWaiting = resendTimer > 0;

    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        onEmailChange(event.target.value);
    };

    const handleSubmit = (event: React.SubmitEvent<HTMLFormElement>) => {
        event.preventDefault();
        onSubmit();
    };

    return (
        <div className="flex w-full flex-col gap-8">
            <StepHeader title="Введите почту" subtitle="На почту придет код для входа" />

            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                <TextField
                    variant="titleIn"
                    title="Электронная почта"
                    name="email"
                    type="email"
                    inputMode="email"
                    autoComplete="email"
                    value={email}
                    onChange={handleChange}
                    onClear={() => onEmailChange("")}
                />

                <Button
                    type="submit"
                    loading={isLoading}
                    disabled={!isEmailValid(email.trim()) || isWaiting}
                >
                    Получить код
                </Button>

                <CooldownNote secondsLeft={resendTimer} />
            </form>

            <SupportRequestButton />
        </div>
    );
}
