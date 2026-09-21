'use client';

import { useEffect, useRef, useState, type ChangeEvent, type JSX, type SubmitEvent } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { notify } from '@/shared/lib/notifications';
import { isValidLoginCode } from '@/shared/lib/login-code';
import {
  formatPhoneDisplay,
  formatPhoneInput,
  normalizePhone,
} from '@/shared/lib/phone';
import {
  Button,
  IconButton,
  PageContent,
  ResendCodeTile,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { MeFlowScreen, MeFlowSuccessScreen, type MeFlowProps } from './MeFlowScreen';
import { useCodeStep } from '../lib/use-code-step';
import {
  useChangePhone,
  useChangePhoneSendCode,
} from '@/features/profile';

function isValidPhone(formatted: string): boolean {
  return normalizePhone(formatted).length === 12;
}

type Step = 'phone' | 'code' | 'success';

/**
 * Экран «Изменение телефона» (карта #591/#723, тикеты #595/#733; Figma
 * 1868-67737 — пустой ввод, 1868-67992 — заполненный, 1869-68137 — код
 * с resend-плиткой, 1869-68303 — успех). Заменяет легаси PhoneChangeForm
 * на маршруте /profile/account/phone. Состояния /me и успех — общий
 * MeFlowScreen. Три шага одного маршрута:
 *  - «phone» — Heading «Введите новый номер телефона», поле «Новый телефон»
 *    (маска +7 (999) 000-00-00), шит с «Продолжить»; крест в шапке
 *    появляется в заполненном состоянии (макет 1868-67992) и закрывает
 *    экран на аккаунт.
 *  - «code» — Heading «Код отправлен на почту» с реальной почтой юзера,
 *    поле «Код» (6 цифр) и resend-канон: плитка «Отправить новый код» с
 *    таймером 60 с (макет 1869-68137); «Назад» возвращает к вводу номера
 *    (таймер флоу живёт и там), крест — отмена потока.
 *  - «success» — новый номер в MeFlowSuccessScreen, шит с «Хорошо»
 *    возвращает на аккаунт.
 * Хедер собирается на TopNav (не SubScreenShell): состав слотов меняется по
 * шагам. Поле шага получает программный фокус на монтировании и смене шага
 * (устоявшийся a11y-паттерн вместо autoFocus). Кнопки шита отправляют
 * формы по атрибуту form — панель живёт вне <form>.
 * Кнопка «Продолжить» именно disabled, пока номер/код неполные, — по
 * макету (1868-67737, состояние Disabled) и тексту тикета; это осознанное
 * исключение из правила §3 «панель скрывается целиком». Ошибки: неверный
 * код (401) — inline в error-проп поля (макет 2343-51004), ввод сбрасывает;
 * остальные API-ошибки — тостами сценариев профиля, текст из detail бэка
 * («Превышен лимит запросов», «Этот номер телефона уже используется»).
 */
export function PhoneChangeScreen(): JSX.Element {
  return <MeFlowScreen title="Изменение телефона" Flow={PhoneChangeFlow} />;
}

function PhoneChangeFlow({ me, onClose }: MeFlowProps): JSX.Element {
  const sendCode = useChangePhoneSendCode();
  const changePhone = useChangePhone();

  const [step, setStep] = useState<Step>('phone');
  const [phone, setPhone] = useState('');
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);
  /** Шаг кода resend-канона (#733): ввод, inline-ошибка 401, дедлайн
   * таймера — от последней успешной отправки, живёт на уровне флоу и
   * переживает «Назад»-переключения шагов. */
  const codeStep = useCodeStep({
    isSubmitAttempted,
    onAttemptReset: () => setIsSubmitAttempted(false),
  });

  // Программный фокус поля шага (jsx-a11y/no-autofocus): точка фокуса
  // остаётся под контролем компонента — паттерн шага кода логина.
  const phoneFieldRef = useRef<HTMLInputElement>(null);
  const codeFieldRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    (step === 'code' ? codeFieldRef : phoneFieldRef).current?.focus();
  }, [step]);

  const normalizedPhone = normalizePhone(phone);
  const isSamePhone = normalizedPhone === me.phone;
  const phoneError = isSubmitAttempted && !isValidPhone(phone)
    ? 'Введите корректный номер телефона'
    : isSamePhone
      ? 'Новый номер совпадает с текущим'
      : undefined;

  const handlePhoneChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setPhone(formatPhoneInput(event.currentTarget.value));
  };

  const handlePhoneClear = (): void => {
    setPhone('');
    setIsSubmitAttempted(false);
  };

  const handleSendCode = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidPhone(phone) || isSamePhone) {
      return;
    }

    sendCode.mutate(
      { phone: normalizedPhone },
      {
        onSuccess: () => {
          codeStep.rearm();
          setStep('code');
        },
        onError: (error) => notify.scenarios.profile.phoneSendCodeError(error),
      },
    );
  };

  /** Плитка «Отправить новый код» (макет 1869-68137): повторная отправка
   * тем же useChangePhoneSendCode; старый код серверно инвалидируется —
   * поле очищается, таймер перезапускается. */
  const handleResend = (): void => {
    sendCode.mutate(
      { phone: normalizedPhone },
      {
        onSuccess: () => codeStep.rearm(),
        onError: (error) => notify.scenarios.profile.phoneSendCodeError(error),
      },
    );
  };

  const handleVerifyCode = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidLoginCode(codeStep.value)) {
      return;
    }

    changePhone.mutate(
      { phone: normalizedPhone, code: codeStep.value },
      {
        onSuccess: () => setStep('success'),
        onError: (error) =>
          codeStep.routeVerifyError(error, notify.scenarios.profile.phoneChangeError),
      },
    );
  };

  /** «Назад» с шага кода — к вводу номера: черновик кода и состояния мутаций
   * сбрасываются; повторная отправка без смены номера — resend-плиткой шага
   * кода, дедлайн таймера флоу сохраняется (#733). */
  const backToPhone = (): void => {
    codeStep.clear();
    setStep('phone');
    sendCode.reset();
    changePhone.reset();
  };

  if (step === 'success') {
    return (
      <MeFlowSuccessScreen
        message={`Новый номер телефона ${formatPhoneDisplay(normalizedPhone)}`}
        onClose={onClose}
      />
    );
  }

  const isCodeStep = step === 'code';

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={isCodeStep ? backToPhone : onClose}
          />
        }
        trailing={
          isCodeStep || phone.length > 0 ? (
            <IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />
          ) : undefined
        }
      >
        <TopNavTitle title="Изменение телефона" />
      </TopNav>

      {isCodeStep ? (
        <PageContent className="px-6">
          <form id="phone-change-code" onSubmit={handleVerifyCode} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <h1 className="m-0 text-xl font-semibold leading-6 text-content">
                Код отправлен на почту
              </h1>
              <p className="m-0 text-sm leading-4 text-content-secondary">
                {me.email !== null
                  ? `Отправили 6-значный код подтверждения на вашу почту ${me.email}`
                  : 'Отправили 6-значный код подтверждения на вашу почту'}
              </p>
            </div>
            <TextField
              variant="titleIn"
              title="Код"
              type="text"
              inputMode="numeric"
              value={codeStep.value}
              onChange={codeStep.handleChange}
              onClear={codeStep.clear}
              error={codeStep.error}
              ref={codeFieldRef}
            />
          </form>
          {/* Resend-канон шага кода (1869-68137): вне формы — кнопка плитки
           * не сабмитит код, зазор от поля 24px по макету. */}
          <ResendCodeTile
            className="mt-6"
            remainingSeconds={codeStep.remainingSeconds}
            loading={sendCode.isPending}
            onResend={handleResend}
          />
        </PageContent>
      ) : (
        <PageContent className="px-6">
          <form id="phone-change-phone" onSubmit={handleSendCode} className="flex flex-col gap-4">
            <h1 className="m-0 text-xl font-semibold leading-6 text-content">
              Введите новый номер телефона
            </h1>
            <TextField
              variant="titleIn"
              title="Новый телефон"
              type="tel"
              inputMode="tel"
              placeholder="+7 (999) 000-00-00"
              value={phone}
              onChange={handlePhoneChange}
              onClear={handlePhoneClear}
              error={phoneError}
              ref={phoneFieldRef}
            />
          </form>
        </PageContent>
      )}

      {/* Кнопка шага — в шите над TabBar, отправляет форму по атрибуту form.
       * В макете шага кода (1869-68137) шит целиком занят клавиатурой и
       * кнопку не показывает — сабмит даёт тот же канонный шит с действием,
       * подпись «Продолжить» как на шаге 1. */}
      <StickyBottomBar>
        {isCodeStep ? (
          <Button
            type="submit"
            form="phone-change-code"
            className="w-full"
            loading={changePhone.isPending}
            disabled={!isValidLoginCode(codeStep.value)}
          >
            Продолжить
          </Button>
        ) : (
          <Button
            type="submit"
            form="phone-change-phone"
            className="w-full"
            loading={sendCode.isPending}
            disabled={!isValidPhone(phone) || isSamePhone}
          >
            Продолжить
          </Button>
        )}
      </StickyBottomBar>
    </>
  );
}
