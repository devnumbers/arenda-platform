"use client";

import {type ChangeEvent, type JSX} from "react";
import {formatPhoneInput, isPhoneValid} from "@/shared/lib/phone";
import {Support} from "@/shared/assets/icons";
import {Button} from "@/shared/ui/button";
import {TextField} from "@/shared/ui/text-field";
import {SendCodeButton} from "@/features/auth/ui/send-code-button";
import styles from "./PhoneStep.module.css";

export type PhoneStepProps = {
    phone: string;
    onPhoneChange: (value: string) => void;
    onSubmit: () => void;
    isLoading: boolean;
    resendTimer?: number;
};

export function PhoneStep({
                              phone,
                              onPhoneChange,
                              onSubmit,
                              isLoading,
                              resendTimer = 0,
                          }: PhoneStepProps): JSX.Element {
    const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
        onPhoneChange(formatPhoneInput(event.target.value));
    };

    const handleSubmit = (event: React.SubmitEvent<HTMLFormElement>) => {
        event.preventDefault();
        onSubmit();
    };

    return (
        <div className={styles.root}>
            <div className={styles.header}>
                <h1 className={styles.title}>Введите номер телефона</h1>
                <p className={styles.subtitle}>Чтобы войти или зарегистрироваться</p>
            </div>

            <form className={styles.form} onSubmit={handleSubmit}>
                <TextField
                    labelPlacement="inside"
                    label="Телефон"
                    placeholder="+7 (000) 000-00-00"
                    value={phone}
                    onChange={handleChange}
                    fullWidth
                />

                <SendCodeButton
                    type="submit"
                    remainingSeconds={resendTimer}
                    loading={isLoading}
                    disabled={!isPhoneValid(phone) || isLoading}
                    timerLabel={() => `Отправить новый код`}
                >
                    Войти
                </SendCodeButton>
            </form>

            <p className={styles.legal}>
                Нажимая кнопку «Войти», я принимаю{" "}
                <a className={styles.legalLink} href="#">
                    политику конфиденциальности
                </a>{" "}
                и{" "}
                <a className={styles.legalLink} href="#">
                    соглашаюсь на обработку персональных данных
                </a>
            </p>

            <Button
                variant="clear"
                size="large"
                fullWidth
                leftIcon={<Support/>}
                onClick={() => {
                    window.location.href = "mailto:support@hatus.ru";
                }}
            >
                Написать в поддержку
            </Button>
        </div>
    );
}
