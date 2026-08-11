"use client";

import {type ReactNode, useCallback, useEffect, useState} from "react";
import clsx from "clsx";
import {ArrowLeft, Cancel, Support} from "@/shared/assets/icons";
import {IconButton} from "@/shared/ui/icon-button";
import {PhoneStep} from "../phone-step/PhoneStep";
import {EmailStep} from "../email-step";
import {CodeStep} from "../code-step/CodeStep";
import styles from "./AuthForm.module.css";

type AuthStep = "phone" | "email" | "code";

export type AuthFormProps = {
    initialStep?: AuthStep;
    step?: AuthStep;
    onStepChange?: (step: AuthStep) => void;
    onSendPhone?: (phone: string) => void;
    onSendEmail?: () => void;
    onVerifyCode?: (code: string) => void;
    onChangePhone?: () => void;
    onChangeEmail?: () => void;
    onResend?: () => void;
    onClose?: () => void;
    /**
     * Hide the "close" (✕) button on the phone step. Used in PWA standalone
     * mode, where there is no landing to return to — closing would push the
     * user out of the app. The back/support buttons on the email/code steps
     * are unaffected.
     */
    hideClose?: boolean;
    phone?: string;
    onPhoneChange?: (value: string) => void;
    email?: string;
    onEmailChange?: (value: string) => void;
    isSending?: boolean;
    isSendingEmail?: boolean;
    isVerifying?: boolean;
    isResending?: boolean;
    resendTimer?: number;
};

function StepTransition({
                            children,
                            stepKey,
                        }: {
    children: ReactNode;
    stepKey: string;
}) {
    const [isActive, setIsActive] = useState(false);

    useEffect(() => {
        const frame = requestAnimationFrame(() => setIsActive(true));
        return () => cancelAnimationFrame(frame);
    }, [stepKey]);

    return (
        <div
            key={stepKey}
            className={clsx("stepEnter", isActive && "stepEnterActive", styles.step)}
        >
            {children}
        </div>
    );
}

export function AuthForm({
                             initialStep = "phone",
                             step: controlledStep,
                             onStepChange,
                             onSendPhone,
                             onSendEmail,
                             onVerifyCode,
                             onChangePhone,
                             onChangeEmail,
                             onResend,
                             onClose,
                             hideClose = false,
                             phone: controlledPhone,
                             onPhoneChange,
                             email: controlledEmail,
                             onEmailChange,
                             isSending = false,
                             isSendingEmail = false,
                             isVerifying = false,
                             isResending = false,
                             resendTimer = 0,
                         }: AuthFormProps) {
    const [internalStep, setInternalStep] = useState<AuthStep>(initialStep);
    const [internalPhone, setInternalPhone] = useState("");
    const [internalEmail, setInternalEmail] = useState("");
    const [code, setCode] = useState("");

    const effectiveStep = controlledStep ?? internalStep;
    const isPhoneControlled = controlledPhone !== undefined;
    const phone = isPhoneControlled ? controlledPhone : internalPhone;
    const isEmailControlled = controlledEmail !== undefined;
    const email = isEmailControlled ? controlledEmail : internalEmail;

    const setStep = useCallback(
        (next: AuthStep) => {
            setInternalStep(next);
            onStepChange?.(next);
        },
        [onStepChange],
    );

    const handlePhoneChange = useCallback(
        (value: string) => {
            if (isPhoneControlled) {
                onPhoneChange?.(value);
            } else {
                setInternalPhone(value);
            }
        },
        [isPhoneControlled, onPhoneChange],
    );

    const handleEmailChange = useCallback(
        (value: string) => {
            if (isEmailControlled) {
                onEmailChange?.(value);
            } else {
                setInternalEmail(value);
            }
        },
        [isEmailControlled, onEmailChange],
    );

    const handleCodeChange = useCallback((value: string) => {
        setCode(value);
    }, []);

    // Clear any entered digits when the step changes (e.g. resend → email → code).
    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setCode("");
    }, [effectiveStep]);

    const handleSendPhone = useCallback(() => {
        onSendPhone?.(phone);
    }, [onSendPhone, phone]);

    const handleSendEmail = useCallback(() => {
        onSendEmail?.();
    }, [onSendEmail]);

    const handleVerifyCode = useCallback(
        (value: string) => {
            onVerifyCode?.(value);
        },
        [onVerifyCode],
    );

    const handleChangePhone = useCallback(() => {
        setStep("phone");
        setCode("");
        onChangePhone?.();
    }, [onChangePhone, setStep]);

    const handleChangeEmail = useCallback(() => {
        setStep("email");
        setCode("");
        onChangeEmail?.();
    }, [onChangeEmail, setStep]);

    const handleResend = useCallback(() => {
        onResend?.();
    }, [onResend]);

    const isEmailFlow = email.length > 0;
    const codeContactType = isEmailFlow ? "email" : "stored-email";
    const codeContact = isEmailFlow ? email : phone;
    const handleCodeBack = isEmailFlow ? handleChangeEmail : handleChangePhone;

    return (
        <div className={styles.root}>
            {effectiveStep === "phone" ? (
                hideClose ? null : (
                    <div className={styles.topBar}>
                        <IconButton
                            variant="primary-icon"
                            size="large"
                            aria-label="Закрыть"
                            icon={<Cancel/>}
                            onClick={onClose}
                            className={styles.iconButton}
                        />
                    </div>
                )
            ) : (
                <div className={clsx(styles.topBar, styles.topBarCode)}>
                    <IconButton
                        variant="primary-icon"
                        size="large"
                        aria-label="Назад"
                        icon={<ArrowLeft/>}
                        onClick={
                            effectiveStep === "email" ? handleChangePhone : handleCodeBack
                        }
                        className={styles.iconButton}
                    />
                    <IconButton
                        variant="primary-icon"
                        size="large"
                        aria-label="Написать в поддержку"
                        icon={<Support/>}
                        onClick={() => {
                            window.location.href = "mailto:support@hatus.ru";
                        }}
                        className={styles.iconButton}
                    />
                </div>
            )}
            {effectiveStep === "phone" && (
                <StepTransition stepKey="phone">
                    <PhoneStep
                        phone={phone}
                        onPhoneChange={handlePhoneChange}
                        onSubmit={handleSendPhone}
                        isLoading={isSending}
                        resendTimer={resendTimer}
                    />
                </StepTransition>
            )}
            {effectiveStep === "email" && (
                <StepTransition stepKey="email">
                    <EmailStep
                        email={email}
                        onEmailChange={handleEmailChange}
                        onSubmit={handleSendEmail}
                        isLoading={isSendingEmail}
                        resendTimer={resendTimer}
                    />
                </StepTransition>
            )}
            {effectiveStep === "code" && (
                <StepTransition stepKey="code">
                    <CodeStep
                        contact={codeContact}
                        contactType={codeContactType}
                        code={code}
                        onCodeChange={handleCodeChange}
                        onVerify={handleVerifyCode}
                        onChangeContact={handleCodeBack}
                        onResend={handleResend}
                        isVerifying={isVerifying}
                        isResending={isResending}
                        resendTimer={resendTimer}
                    />
                </StepTransition>
            )}
        </div>
    );
}
