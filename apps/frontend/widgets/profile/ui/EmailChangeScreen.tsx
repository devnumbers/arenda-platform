'use client';

import { useEffect, useRef, useState, type ChangeEvent, type JSX, type SubmitEvent } from 'react';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { notify } from '@/shared/lib/notifications';
import { isEmailValid } from '@/shared/lib/email';
import { isValidLoginCode, loginCodeFromInput } from '@/shared/lib/login-code';
import {
  Button,
  IconButton,
  PageContent,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { MeFlowScreen, MeFlowSuccessScreen, type MeFlowProps } from './MeFlowScreen';
import {
  useChangeEmail,
  useConfirmCurrentEmail,
  useEmailChangeSendCode,
} from '@/features/profile';

/** Новый адрес: непустой, по форме «что-то@домен.тлд», в нижнем регистре —
 * бэк хранит адрес в lowercase, same-as-current сравнивается с ним же. */
function normalizeEmail(value: string): string {
  return value.trim().toLowerCase();
}

type Step = 'code-current' | 'new-email' | 'code-new' | 'success';

/**
 * Экран «Смена почты» (карта #723, тикет #722; Figma LnnLyFL5u1DWIYPGLzWW0X —
 * 2235-104553 код текущей, 2235-104805/104954 ввод новой пустой/заполненный,
 * 2235-105106 код новой, 2235-105223 успех). Канон — PhoneChangeScreen,
 * состояния /me и успех — общий MeFlowScreen. Четыре шага одного маршрута
 * /profile/account/email:
 *  - «code-current» — код на текущий адрес отправляется автоматически при
 *    входе на экран (шаг «Подтвердите текущую почту» в макете — первый);
 *    ошибка отправки — тост, поле остаётся (живой код из прошлой попытки
 *    принимается — паритет с телефоном). «Назад» и крест закрывают экран.
 *  - «new-email» — «Введите новую почту», поле с крестом-очисткой;
 *    «Продолжить» disabled до валидного адреса; same-as-current —
 *    клиентская ошибка. Здесь же уходит confirm-current (код + адрес одним
 *    запросом, #721): неверный/использованный адрес — тост из detail.
 *  - «code-new» — «Подтвердите новую почту» с новым адресом в подзаголовке.
 *  - «success» — «Электронная почта изменена на <адрес>» (решение #720 Q7:
 *    в макете 105223 стоял телефон), шит «Хорошо» на аккаунт.
 * Шапка «Смена почты» на всех шагах (артефакт макета 104805/104954
 * «Изменение телефона» правим по #720 Q7). Кнопки resend нет (Q8): код на
 * текущий адрес повторно уходит при повторном входе на экран, на новый —
 * через «Назад» → «Продолжить» на шаге адреса.
 * Поле шага получает программный фокус на монтировании и смене шага. Ошибки
 * API — тосты сценариев профиля, текст из detail бэка.
 */
export function EmailChangeScreen(): JSX.Element {
  return <MeFlowScreen title="Смена почты" Flow={EmailChangeFlow} />;
}

function EmailChangeFlow({ me, onClose }: MeFlowProps): JSX.Element {
  const sendCode = useEmailChangeSendCode();
  const confirmCurrent = useConfirmCurrentEmail();
  const changeEmail = useChangeEmail();

  const [step, setStep] = useState<Step>('code-current');
  const [currentCode, setCurrentCode] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newCode, setNewCode] = useState('');
  const [grant, setGrant] = useState('');
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

  // Код на текущую почту уходит один раз на монтирование потока: реф-гард
  // делает пуск идемпотентным (StrictMode и повторные рендеры не дублируют
  // отправку), троттлинг бэка страхует серверно.
  const isSendStartedRef = useRef(false);
  const sendCodeMutate = sendCode.mutate;
  useEffect(() => {
    if (isSendStartedRef.current) {
      return;
    }
    isSendStartedRef.current = true;
    sendCodeMutate(undefined, {
      onError: (error) => notify.scenarios.profile.emailSendCodeError(error),
    });
  }, [sendCodeMutate]);

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
  const currentCodeError = isSubmitAttempted && !isValidLoginCode(currentCode)
    ? 'Введите 6-значный код'
    : undefined;
  const newCodeError = isSubmitAttempted && !isValidLoginCode(newCode)
    ? 'Введите 6-значный код'
    : undefined;

  const handleCodeChange = (
    event: ChangeEvent<HTMLInputElement>,
    setter: (value: string) => void,
  ): void => {
    setter(loginCodeFromInput(event.currentTarget.value));
  };

  /** Шаг 1 → 2 локальный: код проверяется формой, серверу он понадобится на
   * шаге «new-email» (confirm-current несёт код и адрес одним запросом). */
  const handleCurrentCodeSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidLoginCode(currentCode)) {
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
      { newEmail: normalizedNewEmail, code: currentCode },
      {
        onSuccess: ({ grant: newGrant }) => {
          setIsSubmitAttempted(false);
          setGrant(newGrant);
          setNewCode('');
          setStep('code-new');
        },
        onError: (error) => notify.scenarios.profile.emailChangeError(error),
      },
    );
  };

  const handleNewCodeSubmit = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidLoginCode(newCode)) {
      return;
    }

    changeEmail.mutate(
      { grant, code: newCode },
      {
        onSuccess: () => setStep('success'),
        onError: (error) => notify.scenarios.profile.emailChangeError(error),
      },
    );
  };

  /** «Назад» — на шаг раньше: черновик уходящего шага и его мутация
   * сбрасываются. Повторная отправка кода на НОВЫЙ адрес — «Продолжить»
   * шага «new-email» (подтвердит текущий код ещё раз, пока тот жив);
   * код на ТЕКУЩИЙ адрес повторно уходит при повторном входе на экран
   * (закрыть → строка на аккаунте): троттлинг бэка 429 при живом коде —
   * форма остаётся, живой код принимается. */
  const backToPreviousStep = (): void => {
    setIsSubmitAttempted(false);
    if (step === 'new-email') {
      setStep('code-current');
      confirmCurrent.reset();
    } else if (step === 'code-new') {
      setNewCode('');
      setStep('new-email');
      changeEmail.reset();
    } else {
      onClose();
    }
  };

  if (step === 'success') {
    return (
      <MeFlowSuccessScreen
        message={`Электронная почта изменена на ${normalizedNewEmail}`}
        onClose={onClose}
      />
    );
  }

  const isCodeCurrentStep = step === 'code-current';
  const isNewEmailStep = step === 'new-email';
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
                  ? `Отправили 6-значный код подтверждения на почту ${me.email}`
                  : 'Отправили 6-значный код подтверждения на вашу почту'}
              </p>
            </div>
            <TextField
              variant="titleIn"
              title="Код"
              type="text"
              inputMode="numeric"
              value={currentCode}
              onChange={(event) => handleCodeChange(event, setCurrentCode)}
              error={currentCodeError}
              ref={currentCodeRef}
            />
          </form>
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

      {step === 'code-new' && (
        <PageContent className="px-6">
          <form id={formId} onSubmit={handleNewCodeSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <h1 className="m-0 text-xl font-semibold leading-6 text-content">
                Подтвердите новую почту
              </h1>
              <p className="m-0 text-sm leading-4 text-content-secondary">
                Отправили 6-значный код подтверждения на почту {normalizedNewEmail}
              </p>
            </div>
            <TextField
              variant="titleIn"
              title="Код"
              type="text"
              inputMode="numeric"
              value={newCode}
              onChange={(event) => handleCodeChange(event, setNewCode)}
              error={newCodeError}
              ref={newCodeRef}
            />
          </form>
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
            disabled={!isValidLoginCode(currentCode)}
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
        {step === 'code-new' && (
          <Button
            type="submit"
            form={formId}
            className="w-full"
            loading={changeEmail.isPending}
            disabled={!isValidLoginCode(newCode)}
          >
            Продолжить
          </Button>
        )}
      </StickyBottomBar>
    </>
  );
}
