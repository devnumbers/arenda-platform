'use client';

import { useEffect, useRef, useState, type ChangeEvent, type JSX, type SubmitEvent } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { notify } from '@/shared/lib/notifications';
import { isEmailValid } from '@/shared/lib/email';
import { isValidLoginCode } from '@/shared/lib/login-code';
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
  useChangeEmail,
  useConfirmCurrentEmail,
  useEmailChangeSendCode,
  useResendEmailCode,
} from '@/features/profile';

/** Новый адрес: непустой, по форме «что-то@домен.тлд», в нижнем регистре —
 * бэк хранит адрес в lowercase, same-as-current сравнивается с ним же. */
function normalizeEmail(value: string): string {
  return value.trim().toLowerCase();
}

type Step = 'code-current' | 'new-email' | 'code-new' | 'success';

/**
 * Экран «Смена почты» (карта #723, тикеты #722/#733; Figma
 * LnnLyFL5u1DWIYPGLzWW0X — 2235-104553 код текущей, 2235-104805/104954
 * ввод новой пустой/заполненный, 2235-105106 код новой, 2343-51004
 * ошибка кода/истёкший таймер, 105223 успех). Канон — PhoneChangeScreen,
 * состояния /me и успех — общий MeFlowScreen. Четыре шага одного маршрута
 * /profile/account/email:
 *  - «code-current» — код на текущий адрес отправляется автоматически при
 *    входе на экран (шаг «Подтвердите текущую почту» в макете — первый);
 *    ошибка отправки — тост, поле остаётся. Resend — плитка с таймером
 *    60 с (повторный useEmailChangeSendCode). «Назад» и крест закрывают
 *    экран.
 *  - «new-email» — «Введите новую почту», поле с крестом-очисткой;
 *    «Продолжить» disabled до валидного адреса; same-as-current —
 *    клиентская ошибка. Здесь же уходит confirm-current (код + адрес одним
 *    запросом, #721): неверный/использованный адрес — тост из detail.
 *  - «code-new» — «Подтвердите новую почту» с новым адресом в подзаголовке;
 *    resend — плитка с таймером, повторная доставка по живому гранту
 *    (`useResendEmailCode`, бэк #732): код шага 1 confirm-current сжигает,
 *    прежний обход «Назад → Продолжить» серверно сломан.
 *  - «success» — «Новая электронная почта <адрес>» (макет 105223),
 *    шит «Хорошо» на аккаунт.
 * Шапка «Смена почты» на всех шагах. Поле шага получает программный фокус
 * на монтировании и смене шага. Ошибки: неверный код (401 verify-мутации
 * changeEmail шага «code-new») — inline в error-проп поля (макет
 * 2343-51004), ввод сбрасывает; 401 confirm-current (неверный код шага 1
 * на шаге «new-email») и остальные API-ошибки — тосты сценариев профиля,
 * текст из detail бэка. Дедлайны resend-таймеров — на уровне флоу,
 * переживают «Назад»-переключения.
 */
export function EmailChangeScreen(): JSX.Element {
  return <MeFlowScreen title="Смена почты" Flow={EmailChangeFlow} />;
}

function EmailChangeFlow({ me, onClose }: MeFlowProps): JSX.Element {
  const sendCode = useEmailChangeSendCode();
  const confirmCurrent = useConfirmCurrentEmail();
  const changeEmail = useChangeEmail();
  const resendEmailCode = useResendEmailCode();

  const [step, setStep] = useState<Step>('code-current');
  const [newEmail, setNewEmail] = useState('');
  const [grant, setGrant] = useState('');
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);
  /** Шаги кода resend-канона (#733) — экземпляр на шаг: ввод, inline-ошибка
   * 401, дедлайн таймера от последней отправки; дедлайны живут на уровне
   * флоу и переживают «Назад»-переключения. */
  const currentCodeStep = useCodeStep({
    isSubmitAttempted,
    onAttemptReset: () => setIsSubmitAttempted(false),
  });
  const newCodeStep = useCodeStep({
    isSubmitAttempted,
    onAttemptReset: () => setIsSubmitAttempted(false),
  });
  const { restartCooldown: restartCurrentCooldown } = currentCodeStep;

  // Код на текущую почту уходит один раз на монтирование потока: реф-гард
  // делает пуск идемпотентным (StrictMode и повторные рендеры не дублируют
  // отправку), троттлинг бэка страхует серверно. Успех — старт таймера
  // resend-плитки шага.
  const isSendStartedRef = useRef(false);
  const sendCodeMutate = sendCode.mutate;
  useEffect(() => {
    if (isSendStartedRef.current) {
      return;
    }
    isSendStartedRef.current = true;
    sendCodeMutate(undefined, {
      onSuccess: () => restartCurrentCooldown(),
      onError: (error) => notify.scenarios.profile.emailSendCodeError(error),
    });
  }, [sendCodeMutate, restartCurrentCooldown]);

  // Программный фокус поля шага (jsx-a11y/no-autofocus): точка фокуса
  // остаётся под контролем компонента — паттерн шага кода логина.
  const currentCodeRef = useRef<HTMLInputElement>(null);
  const newEmailRef = useRef<HTMLInputElement>(null);
  const newCodeRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (step === 'code-current') {
      currentCodeRef.current?.focus();
    } else if (step === 'new-email') {
      newEmailRef.current?.focus();
    } else if (step === 'code-new') {
      newCodeRef.current?.focus();
    }
  }, [step]);

  const normalizedNewEmail = normalizeEmail(newEmail);
  const isSameEmail = normalizedNewEmail !== '' && normalizedNewEmail === (me.email ?? '').toLowerCase();
  const newEmailError = isSubmitAttempted && !isEmailValid(normalizedNewEmail)
    ? 'Введите корректную электронную почту'
    : isSameEmail
      ? 'Новый адрес совпадает с текущим'
      : undefined;

  /** Шаг 1 → 2 локальный: код проверяется формой, серверу он понадобится на
   * шаге «new-email» (confirm-current несёт код и адрес одним запросом). */
  const handleCurrentCodeSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidLoginCode(currentCodeStep.value)) {
      return;
    }

    setIsSubmitAttempted(false);
    setStep('new-email');
  };

  const handleNewEmailChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setNewEmail(event.currentTarget.value);
  };

  const handleNewEmailClear = (): void => {
    setNewEmail('');
    setIsSubmitAttempted(false);
  };

  const handleNewEmailSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isEmailValid(normalizedNewEmail) || isSameEmail) {
      return;
    }

    confirmCurrent.mutate(
      { newEmail: normalizedNewEmail, code: currentCodeStep.value },
      {
        onSuccess: ({ grant: newGrant }) => {
          setGrant(newGrant);
          newCodeStep.rearm();
          setStep('code-new');
        },
        onError: (error) => notify.scenarios.profile.emailChangeError(error),
      },
    );
  };

  const handleNewCodeSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidLoginCode(newCodeStep.value)) {
      return;
    }

    changeEmail.mutate(
      { grant, code: newCodeStep.value },
      {
        onSuccess: () => setStep('success'),
        onError: (error) =>
          newCodeStep.routeVerifyError(error, notify.scenarios.profile.emailChangeError),
      },
    );
  };

  /** Плитка resend на шаге «code-current»: повторная отправка кода на
   * текущий адрес тем же useEmailChangeSendCode; поле очищается, таймер
   * перезапускается. */
  const handleResendCurrentCode = (): void => {
    sendCode.mutate(undefined, {
      onSuccess: () => currentCodeStep.rearm(),
      onError: (error) => notify.scenarios.profile.emailSendCodeError(error),
    });
  };

  /** Плитка resend на шаге «code-new» (#732): повторная доставка по живому
   * гранту — код шага 1 сожжён, другой пути нет; старый код нового адреса
   * серверно инвалидируется — поле очищается, таймер перезапускается. */
  const handleResendNewCode = (): void => {
    resendEmailCode.mutate(
      { grant },
      {
        onSuccess: () => newCodeStep.rearm(),
        onError: (error) => notify.scenarios.profile.emailSendCodeError(error),
      },
    );
  };

  /** «Назад» — на шаг раньше: черновик уходящего шага и его мутация
   * сбрасываются. Повторная отправка кода на НОВЫЙ адрес — resend-плитка
   * шага «code-new»: confirm-current сжигает код шага 1, повторное
   * «Продолжить» с шага адреса даёт 401. Код на ТЕКУЩИЙ адрес повторно
   * уходит плиткой шага 1 (или при повторном входе на экран): троттлинг
   * бэка 429 при живом коде — форма остаётся. */
  const backToPreviousStep = (): void => {
    setIsSubmitAttempted(false);
    if (step === 'new-email') {
      setStep('code-current');
      confirmCurrent.reset();
    } else if (step === 'code-new') {
      newCodeStep.clear();
      setStep('new-email');
      changeEmail.reset();
    } else {
      onClose();
    }
  };

  if (step === 'success') {
    return (
      <MeFlowSuccessScreen
        message={`Новая электронная почта ${normalizedNewEmail}`}
        onClose={onClose}
      />
    );
  }

  const isCodeCurrentStep = step === 'code-current';
  const isNewEmailStep = step === 'new-email';
  const isNewCodeStep = step === 'code-new';
  const formId = `email-change-${step}`;

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={backToPreviousStep}
          />
        }
        trailing={
          <IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />
        }
      >
        <TopNavTitle title="Смена почты" />
      </TopNav>

      {isCodeCurrentStep && (
        <PageContent className="px-6">
          <form id={formId} onSubmit={handleCurrentCodeSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <h1 className="m-0 text-xl font-semibold leading-6 text-content">
                Подтвердите текущую почту
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
              value={currentCodeStep.value}
              onChange={currentCodeStep.handleChange}
              onClear={currentCodeStep.clear}
              error={currentCodeStep.error}
              ref={currentCodeRef}
            />
          </form>
          {/* Resend-канон шага кода (2235-104553): вне формы — кнопка плитки
           * не сабмитит код, зазор от поля 24px по макету. */}
          <ResendCodeTile
            className="mt-6"
            remainingSeconds={currentCodeStep.remainingSeconds}
            loading={sendCode.isPending}
            onResend={handleResendCurrentCode}
          />
        </PageContent>
      )}

      {isNewEmailStep && (
        <PageContent className="px-6">
          <form id={formId} onSubmit={handleNewEmailSubmit} className="flex flex-col gap-4">
            <h1 className="m-0 text-xl font-semibold leading-6 text-content">
              Введите новую почту
            </h1>
            <TextField
              variant="titleIn"
              title="Электронная почта"
              type="email"
              inputMode="email"
              autoComplete="email"
              value={newEmail}
              onChange={handleNewEmailChange}
              onClear={handleNewEmailClear}
              error={newEmailError}
              ref={newEmailRef}
            />
          </form>
        </PageContent>
      )}

      {isNewCodeStep && (
        <PageContent className="px-6">
          <form id={formId} onSubmit={handleNewCodeSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <h1 className="m-0 text-xl font-semibold leading-6 text-content">
                Подтвердите новую почту
              </h1>
              <p className="m-0 text-sm leading-4 text-content-secondary">
                Отправили 6-значный код подтверждения на вашу почту {normalizedNewEmail}
              </p>
            </div>
            <TextField
              variant="titleIn"
              title="Код"
              type="text"
              inputMode="numeric"
              value={newCodeStep.value}
              onChange={newCodeStep.handleChange}
              onClear={newCodeStep.clear}
              error={newCodeStep.error}
              ref={newCodeRef}
            />
          </form>
          {/* Resend-канон шага кода (2235-105106, 2343-51004): повторная
           * доставка по гранту — бэк #732; вне формы, зазор 24px. */}
          <ResendCodeTile
            className="mt-6"
            remainingSeconds={newCodeStep.remainingSeconds}
            loading={resendEmailCode.isPending}
            onResend={handleResendNewCode}
          />
        </PageContent>
      )}

      {/* Кнопка шага — в шите над TabBar, отправляет форму по атрибуту form.
       * В макетах кода (104553, 105106) шит целиком занят клавиатурой —
       * сабмит даёт тот же канонный шит с действием. «Продолжить» именно
       * disabled, пока поле неполное/невалидное, — по макетам 104805/104954
       * и канону телефонного флоу. */}
      <StickyBottomBar>
        {isCodeCurrentStep && (
          <Button
            type="submit"
            form={formId}
            className="w-full"
            disabled={!isValidLoginCode(currentCodeStep.value)}
          >
            Продолжить
          </Button>
        )}
        {isNewEmailStep && (
          <Button
            type="submit"
            form={formId}
            className="w-full"
            loading={confirmCurrent.isPending}
            disabled={!isEmailValid(normalizedNewEmail) || isSameEmail}
          >
            Продолжить
          </Button>
        )}
        {isNewCodeStep && (
          <Button
            type="submit"
            form={formId}
            className="w-full"
            loading={changeEmail.isPending}
            disabled={!isValidLoginCode(newCodeStep.value)}
          >
            Продолжить
          </Button>
        )}
      </StickyBottomBar>
    </>
  );
}
