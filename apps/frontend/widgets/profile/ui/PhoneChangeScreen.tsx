'use client';

import { useEffect, useRef, useState, type ChangeEvent, type JSX, type SubmitEvent } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  formatPhoneDisplay,
  formatPhoneInput,
  normalizePhone,
} from '@/shared/lib/phone';
import {
  Button,
  IconButton,
  PageContent,
  Skeleton,
  StatusIcon,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { useMe } from '@/features/auth';
import type { User } from '@/entities/user';
import {
  useChangePhone,
  useChangePhoneSendCode,
} from '@/features/profile';

const CODE_LENGTH = 6;

function isValidPhone(formatted: string): boolean {
  return normalizePhone(formatted).length === 12;
}

function isValidCode(value: string): boolean {
  return /^\d{6}$/.test(value);
}

type Step = 'phone' | 'code' | 'success';

/**
 * Экран «Изменение телефона» (карта #591, тикет #595; Figma 1868-67737 —
 * пустой ввод, 1868-67992 — заполненный, 1869-68137 — код, 1869-68303 —
 * успех). Заменяет легаси PhoneChangeForm на маршруте
 * /profile/account/phone. Три шага одного маршрута:
 *  - «phone» — Heading «Введите новый номер телефона», поле «Новый телефон»
 *    (маска +7 (999) 000-00-00), шит с «Продолжить»; крест в шапке
 *    появляется в заполненном состоянии (макет 1868-67992) и закрывает
 *    экран на аккаунт.
 *  - «code» — Heading «Код отправлен на почту» с реальной почтой юзера,
 *    поле «Код» (6 цифр); «Назад» возвращает к вводу номера (и служит
 *    повторной отправкой через новый «Продолжить»), крест — отмена потока.
 *  - «success» — галка GoodWhite 96 (нода 671:6753) и новый номер, шит с
 *    «Хорошо» возвращает на аккаунт; в шапке — только крест (макет без
 *    заголовка). Тост успеха не дублируется — экран сам подтверждение.
 * Хедер собирается на TopNav (не SubScreenShell): состав слотов меняется по
 * шагам. Поле шага получает программный фокус на монтировании и смене шага
 * (устоявшийся a11y-паттерн вместо autoFocus). Кнопки шита отправляют
 * формы по атрибуту form — панель живёт вне <form>.
 * Кнопка «Продолжить» именно disabled, пока номер/код неполные, — по
 * макету (1868-67737, состояние Disabled) и тексту тикета; это осознанное
 * исключение из правила §3 «панель скрывается целиком». Ошибки API —
 * тостами сценариев профиля: текст берётся из detail бэка («Неверный код»,
 * «Превышен лимит запросов», «Этот номер телефона уже используется»).
 */
export function PhoneChangeScreen(): JSX.Element {
  const router = useRouter();
  const { data: me, isPending: isMeLoading, isError: isMeError, refetch } = useMe();

  const close = (): void => goBack(router, ROUTES.profileAccount);

  return (
    <>
      {isMeError && (
        <>
          <StaticHeader onClose={close} />
          <PageContent className="px-6">
            <div className="flex flex-col items-center gap-4 rounded-3xl bg-surface-muted px-6 py-8">
              <p className="m-0 text-sm text-content-secondary">Не удалось загрузить данные</p>
              <Button variant="white" onClick={() => void refetch()}>
                Повторить
              </Button>
            </div>
          </PageContent>
        </>
      )}
      {!isMeError && isMeLoading && (
        <>
          <StaticHeader onClose={close} />
          <PageContent className="px-6">
            {/* Скелетон — форма шага 1 (§7 DESIGN.md): заголовок + бокс поля,
             * внизу — шит с «кнопкой». */}
            <div role="status" aria-label="Загрузка" className="flex flex-col gap-4">
              <Skeleton className="h-6 w-3/4" />
              <Skeleton className="h-14 w-full rounded-button" />
            </div>
          </PageContent>
          <StickyBottomBar>
            <Skeleton className="h-14 w-full rounded-button" />
          </StickyBottomBar>
        </>
      )}
      {!isMeError && !isMeLoading && (
        <PhoneChangeFlow me={me} onClose={close} />
      )}
    </>
  );
}

/** Шапка состояний загрузки/ошибки: «Назад» на аккаунт, заголовок — как у
 * шагов потока (истории может не быть — goBack с фолбэком). */
function StaticHeader({ onClose }: { readonly onClose: () => void }): JSX.Element {
  return (
    <TopNav
      leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />}
    >
      <TopNavTitle title="Изменение телефона" />
    </TopNav>
  );
}

function PhoneChangeFlow({
  me,
  onClose,
}: {
  readonly me: User;
  readonly onClose: () => void;
}): JSX.Element {
  const sendCode = useChangePhoneSendCode();
  const changePhone = useChangePhone();

  const [step, setStep] = useState<Step>('phone');
  const [phone, setPhone] = useState('');
  const [code, setCode] = useState('');
  const [isSubmitAttempted, setIsSubmitAttempted] = useState(false);

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
  const codeError = isSubmitAttempted && !isValidCode(code)
    ? 'Введите 6-значный код'
    : undefined;

  const handlePhoneChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setPhone(formatPhoneInput(event.currentTarget.value));
  };

  const handlePhoneClear = (): void => {
    setPhone('');
    setIsSubmitAttempted(false);
  };

  const handleCodeChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setCode(event.currentTarget.value.replace(/\D/g, '').slice(0, CODE_LENGTH));
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
          setIsSubmitAttempted(false);
          setCode('');
          setStep('code');
        },
        onError: (error) => notify.scenarios.profile.phoneSendCodeError(error),
      },
    );
  };

  const handleVerifyCode = (event: SubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    setIsSubmitAttempted(true);

    if (!isValidCode(code)) {
      return;
    }

    changePhone.mutate(
      { phone: normalizedPhone, code },
      {
        onSuccess: () => setStep('success'),
        onError: (error) => notify.scenarios.profile.phoneChangeError(error),
      },
    );
  };

  /** «Назад» с шага кода — к вводу номера: черновик кода и состояния мутаций
   * сбрасываются (повторная отправка — новый «Продолжить» на шаге 1). */
  const backToPhone = (): void => {
    setStep('phone');
    setCode('');
    setIsSubmitAttempted(false);
    sendCode.reset();
    changePhone.reset();
  };

  if (step === 'success') {
    return (
      <>
        <TopNav
          leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />}
        />
        <PageContent>
          <div className="flex flex-col items-center px-6 pt-16">
            <StatusIcon status="good" className="h-24 w-24" />
            <h1 className="m-0 mt-10 text-center text-xl font-semibold leading-6 text-content">
              Новый номер телефона {formatPhoneDisplay(normalizedPhone)}
            </h1>
          </div>
        </PageContent>
        <StickyBottomBar>
          <Button className="w-full" onClick={onClose}>
            Хорошо
          </Button>
        </StickyBottomBar>
      </>
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
              value={code}
              onChange={handleCodeChange}
              error={codeError}
              ref={codeFieldRef}
            />
          </form>
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
            disabled={!isValidCode(code)}
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
