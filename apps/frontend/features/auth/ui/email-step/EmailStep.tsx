'use client';

import { type ChangeEvent, type JSX } from "react";
import { isEmailValid } from "@/shared/lib/email";
import { formatCountdown } from "@/shared/lib/countdown";
import { Support } from "@/shared/assets/icons";
import { Button, HeaderLogo, TextField } from "@/shared/ui/design";

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
 * почту. «Написать в поддержку» — заглушка mailto до модалки поддержки
 * (#766); почта hello@rentlee.ru — решение владельца 18.09 (карта #761). */
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
            <HeaderLogo className="h-7 w-28" />

            <div className="flex flex-col gap-2">
                <h1 className="m-0 text-[28px] font-semibold leading-8 text-content">
                    Введите почту
                </h1>
                <p className="m-0 text-base leading-[18px] text-content-secondary">
                    На почту придет код для входа
                </p>
            </div>

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

                {isWaiting && (
                    <p className="m-0 text-center font-mono text-sm font-medium leading-4 text-content-tertiary">
                        Запросить код можно через {formatCountdown(resendTimer)}
                    </p>
                )}
            </form>

            <Button
                variant="white"
                className="w-full"
                trailingIcon={<Support />}
                onClick={() => {
                    window.location.href = "mailto:hello@rentlee.ru";
                }}
            >
                Написать в поддержку
            </Button>
        </div>
    );
}
