'use client';

import { type ChangeEvent, type JSX } from "react";
import { formatPhoneInput, isPhoneValid } from "@/shared/lib/phone";
import { Button, ResendCodeTile, TextField } from "@/shared/ui/design";
import { ROUTES } from "@/shared/config/routes";
import { StepHeader, SupportRequestButton } from "../step-chrome";

export type PhoneStepProps = {
    phone: string;
    onPhoneChange: (value: string) => void;
    onSendPhone: (formattedPhone: string) => void;
    isLoading: boolean;
    resendTimer?: number;
};

/** Шаг «Введите номер телефона» по макетам Рентли (карта #761, тикет
 * #763; Figma 2343:66417 — десктоп-экран, блок формы 2343:66422,
 * 2343:67172/2349:67759 — мобайл, пустое/заполненное). Колонка блоков
 * с зазором 32: лого-вектор 112×28 —
 * канон HeaderLogo; заголовок H1 28/32 + подпись 16/18 (зазор 8); поле —
 * канон TextField titleIn (плавающий лейбл, крестик очистки при значении)
 * + Primary-кнопка (зазор 16); юртекст 16/18 tertiary с подчёркнутыми
 * ссылками; «Написать в поддержку» — White-пилюля с Icon/R/Support.
 * Кнопка в покое погашена (disabled 50% = #95BFFF макета). Кулдаун
 * 429/retryAfter и возврат с код-шага с живым таймером — кнопка шага
 * меняется на плитку resend-канона (#733) ResendCodeTile: Secondary
 * «Отправить новый код», погашенная, с двухстрочной подписью
 * «Запросить новый код можно / через ММ:СС» (решение владельца 12.10,
 * макеты 2349:67594/67633); по истечении — снова Primary. Юрссылки
 * ведут на лендинг (/privacy, /terms) в новой вкладке — самих страниц пока
 * нет, 404 лендинга не блокирует (решение владельца 18.09). «Написать в
 * поддержку» открывает модалку «Связаться с нами» (#766). Кнопка погашена
 * и на время отправки кода (решение владельца #1099) — без двойного
 * POST-а и ловящего 429 второго клика. */
export function PhoneStep({
    phone,
    onPhoneChange,
    onSendPhone,
    isLoading,
    resendTimer = 0,
}: PhoneStepProps): JSX.Element {
    const isWaiting = resendTimer > 0;

    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        onPhoneChange(formatPhoneInput(event.target.value));
    };

    const handleSubmit = (event: React.SubmitEvent<HTMLFormElement>) => {
        event.preventDefault();
        // В кулдауне сабмитной кнопки в форме нет (плитка — type="button"),
        // а однофилдовая форма сабмитится по Enter и без неё: гард держит
        // старое поведение — вместо заведомого 429 Enter в кулдауне молчит.
        if (isWaiting) {
            return;
        }
        onSendPhone(phone);
    };

    return (
        <div className="flex w-full flex-col gap-8">
            <StepHeader
                title="Введите номер телефона"
                subtitle="Чтобы войти или зарегистрироваться"
            />

            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                <TextField
                    variant="titleIn"
                    title="Телефон"
                    name="phone"
                    type="tel"
                    inputMode="tel"
                    autoComplete="tel"
                    enterKeyHint="go"
                    value={phone}
                    onChange={handleChange}
                    onClear={() => onPhoneChange("")}
                />

                {/* isLoading дублируется в disabled: Button резолвит
                    disabled ?? loading, и явный disabled без || не даёт
                    loading диспейблить кнопку — на время отправки она
                    оставалась активной (#1099). */}
                {/* В кулдауне слот кнопки занимает плитка канона: она видна
                    только погашенной (isWaiting true весь её жизнь — на нуле
                    слот возвращается Primary), а Enter в кулдауне гасится
                    гардом handleSubmit — заведомого 429 нет. */}
                {isWaiting ? (
                    <ResendCodeTile
                        remainingSeconds={resendTimer}
                        loading={isLoading}
                        onResend={() => onSendPhone(phone)}
                    />
                ) : (
                    <Button
                        type="submit"
                        loading={isLoading}
                        disabled={!isPhoneValid(phone) || isLoading}
                    >
                        Войти
                    </Button>
                )}
            </form>

            <p className="m-0 text-base leading-[18px] text-content-tertiary">
                Нажимая кнопку «Войти», я принимаю{" "}
                <a
                    className="underline"
                    href={ROUTES.landingPrivacy}
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    политику конфиденциальности
                </a>{" "}
                и{" "}
                <a
                    className="underline"
                    href={ROUTES.landingTerms}
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    соглашаюсь на обработку персональных данных
                </a>
            </p>

            <SupportRequestButton />
        </div>
    );
}
