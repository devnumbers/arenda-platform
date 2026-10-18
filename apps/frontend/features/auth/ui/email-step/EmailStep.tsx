'use client';

import { type ChangeEvent, type JSX } from "react";
import { isEmailValid } from "@/shared/lib/email";
import { Button, ResendCodeTile, TextField } from "@/shared/ui/design";
import { StepHeader, SupportRequestButton } from "../step-chrome";

export type EmailStepProps = {
    email: string;
    onEmailChange: (value: string) => void;
    onSubmit: () => void;
    isLoading: boolean;
    resendTimer?: number;
};

/** Шаг «Введите почту» по макетам Рентли (карта #761, тикет #764; Figma
 * 2349:67698 — мобайл, пустое, 2349:67820 — заполненное; десктоп/планшет
 * в файле не нарисованы — собраны по шейлу телефона #763; приёмка 19.09
 * приняла). Колонка блоков с зазором 32: лого-вектор 112×28 — канон
 * HeaderLogo; заголовок H1 28/32 + подпись 16/18 (зазор 8); поле — канон
 * TextField titleIn (плавающий лейбл «Электронная почта», крестик очистки
 * при значении) + Primary-кнопка (зазор 16); «Написать в поддержку» —
 * White-пилюля с Icon/R/Support, как на шаге телефона. Юртекста на макете
 * почты нет — consent дан на шаге телефона. Кнопка в покое погашена
 * (disabled до валидной почты; isEmailValid — та же свободная форма
 * «что-то@домен.тлд», что на бэкенде). Кулдаун 429/retryAfter и возврат
 * с код-шага с живым таймером — кнопка шага меняется на плитку
 * resend-канона (#733) ResendCodeTile: Secondary «Отправить новый код»,
 * погашенная, с двухстрочной подписью «Запросить новый код можно /
 * через ММ:СС» (решение владельца 12.10, макеты 2349:67594/67633);
 * по истечении — снова Primary. Шаг видят только новые пользователи
 * (sent:false из POST /auth/send, ADR 0015) — код уходит на введённую
 * почту. Кнопка погашена и на время отправки (решение владельца #1099) —
 * как на шаге телефона. «Написать в поддержку» открывает модалку
 * «Связаться с нами» (#766). Стрелка ← в баре шейла — на телефон
 * (#1101: добавлена словом владельца, в кадрах её нет; черновик почты
 * при возврате сохраняется). */
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
        // В кулдауне сабмитной кнопки в форме нет (плитка — type="button"),
        // а однофилдовая форма сабмитится по Enter и без неё: гард держит
        // старое поведение — вместо заведомого 429 Enter в кулдауне молчит.
        if (isWaiting) {
            return;
        }
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
                    enterKeyHint="go"
                    value={email}
                    onChange={handleChange}
                    onClear={() => onEmailChange("")}
                />

                {/* isLoading дублируется в disabled — канал тот же, что на
                    шаге телефона (#1099): Button резолвит disabled ?? loading. */}
                {/* В кулдауне слот кнопки занимает плитка канона: она видна
                    только погашенной (isWaiting true весь её жизнь — на нуле
                    слот возвращается Primary), а Enter в кулдауне гасится
                    гардом handleSubmit — заведомого 429 нет. */}
                {isWaiting ? (
                    <ResendCodeTile
                        remainingSeconds={resendTimer}
                        loading={isLoading}
                        onResend={onSubmit}
                    />
                ) : (
                    <Button
                        type="submit"
                        loading={isLoading}
                        disabled={!isEmailValid(email.trim()) || isLoading}
                    >
                        Получить код
                    </Button>
                )}
            </form>

            <SupportRequestButton />
        </div>
    );
}
