'use client';

import { useCallback, useEffect, useRef, useState, type JSX } from 'react';
import NextLink from 'next/link';
import { SmallArrowDown, SmallArrowRight } from '@/shared/assets/icons';
import { Button, Skeleton, TextField } from '@/shared/ui/design';
import { useMe } from '@/features/auth';
import { useUpdateMe } from '@/features/profile';
import type { User } from '@/entities/user';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';
import { formatPhoneDisplay } from '@/shared/lib/phone';
import { notify } from '@/shared/lib/notifications';
import { formatTimezoneLabel } from '../lib/timezone';
import { isEmailValid, profileFieldPatch, type ProfileTextField } from '../lib/profile-edit';
import { AvatarPlaceholder } from './AvatarPlaceholder';

/** Серый бокс поля (как у канонных TextField/PickerField): строка-поле
 * «Телефон» и «Часовой пояс» экрана (Figma 1789-99036 — Input Field
 * 1218:53539 Title Out с иконкой в хвосте). */
const fieldBoxClass = cn(
  'flex h-14 w-full items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2',
);

function FieldTitle({ children }: { readonly children: string }): JSX.Element {
  return (
    <span className="text-base font-medium leading-[18px] text-content">{children}</span>
  );
}

function FieldTailIcon({ children }: { readonly children: JSX.Element }): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
    >
      {children}
    </span>
  );
}

/** Строка «Телефон» (Figma 1789-99036): значение + SmallArrowRight, ведёт
 * на флоу смены телефона (переработка — тикет #595). */
function PhoneFieldRow({ phone }: { readonly phone: string }): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <FieldTitle>Телефон</FieldTitle>
      <NextLink
        href={ROUTES.profileChangePhone}
        className={cn(
          fieldBoxClass,
          'cursor-pointer transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
        )}
      >
        <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
          {phone}
        </span>
        <FieldTailIcon>
          <SmallArrowRight className="h-6 w-6" />
        </FieldTailIcon>
      </NextLink>
    </div>
  );
}

/** Строка «Часовой пояс» (Figma 1789-99036): значение + SmallArrowDown.
 * Пока только показывает сохранённую зону — пикер откроется в тикете
 * #594 (тут станет PickerField), поэтому строка неинтерактивна. */
function TimezoneFieldRow({ label }: { readonly label: string }): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <FieldTitle>Часовой пояс</FieldTitle>
      <div className={fieldBoxClass}>
        <span
          className={cn(
            'min-w-0 flex-1 truncate text-base leading-[18px]',
            label === '' ? 'text-content-secondary' : 'text-content',
          )}
        >
          {label === '' ? 'Не выбран' : label}
        </span>
        <FieldTailIcon>
          <SmallArrowDown className="h-6 w-6" />
        </FieldTailIcon>
      </div>
    </div>
  );
}

function AccountScreenSkeleton(): JSX.Element {
  return (
    <div role="status" aria-label="Загрузка аккаунта" className="flex flex-col gap-6 pb-6">
      <div className="flex justify-center">
        <Skeleton className="h-24 w-24 rounded-pill" />
      </div>
      <div className="flex flex-col gap-8">
        <div className="flex flex-col gap-2">
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-14 w-full rounded-button" />
          <Skeleton className="h-14 w-full rounded-button" />
          <Skeleton className="h-14 w-full rounded-button" />
        </div>
        {['w-20', 'w-40', 'w-28'].map((width) => (
          <div key={width} className="flex flex-col gap-2">
            <Skeleton className={cn('h-5', width)} />
            <Skeleton className="h-14 w-full rounded-button" />
          </div>
        ))}
      </div>
    </div>
  );
}

type AccountScreenViewProps = {
  readonly me: User;
};

function AccountScreenView({ me }: AccountScreenViewProps): JSX.Element {
  const updateMe = useUpdateMe();
  const [name, setName] = useState(me.name ?? '');
  const [surname, setSurname] = useState(me.surname ?? '');
  const [patronymic, setPatronymic] = useState(me.patronymic ?? '');
  const [email, setEmail] = useState(me.email ?? '');
  const [emailError, setEmailError] = useState<string | undefined>(undefined);

  /** Тихое автосохранение (макет без кнопки «Сохранить»): поле коммитится
   * на blur и крестиком очистки — патч только этого поля; пустая строка
   * очищает значение (контракт бэка: null = «не менять»). Невалидная почта
   * не отправляется. */
  const commitField = useCallback(
    (field: ProfileTextField, rawValue: string) => {
      if (field === 'email') {
        const value = rawValue.trim();
        if (value === '') {
          // Бэк очистку почты не принимает (NewEmail('') → 400, адрес —
          // канал входа и уведомлений): черновик откатывается к сохранённому.
          setEmail(me.email ?? '');
          setEmailError(undefined);
          return;
        }
        if (!isEmailValid(value)) {
          setEmailError('Введите корректный email');
          return;
        }
      }
      setEmailError(undefined);
      const patch = profileFieldPatch(me, field, rawValue);
      if (patch === null) {
        return;
      }
      updateMe.mutate(patch, {
        onSuccess: () => notify.scenarios.profile.personalDataSaved(),
        onError: (error) => notify.scenarios.profile.personalDataSaveError(error),
      });
    },
    [me, updateMe],
  );

  const commitName = useCallback(
    (field: Exclude<ProfileTextField, 'email'>) => {
      if (field === 'name') {
        commitField('name', name);
      } else if (field === 'surname') {
        commitField('surname', surname);
      } else {
        commitField('patronymic', patronymic);
      }
    },
    [commitField, name, surname, patronymic],
  );

  return (
    <div className="flex flex-col gap-6 pb-6">
      <div className="flex justify-center">
        <AvatarPlaceholder />
      </div>
      <div className="flex flex-col gap-8">
        <section aria-label="Данные" className="flex flex-col gap-2">
          <h2 className="m-0 text-base font-medium leading-[18px] text-content">Данные</h2>
          <TextField
            variant="titleIn"
            title="Имя"
            value={name}
            onChange={(event) => setName(event.currentTarget.value)}
            onBlur={() => commitName('name')}
            onClear={() => {
              setName('');
              commitField('name', '');
            }}
          />
          <TextField
            variant="titleIn"
            title="Фамилия"
            value={surname}
            onChange={(event) => setSurname(event.currentTarget.value)}
            onBlur={() => commitName('surname')}
            onClear={() => {
              setSurname('');
              commitField('surname', '');
            }}
          />
          <TextField
            variant="titleIn"
            title="Отчество"
            value={patronymic}
            onChange={(event) => setPatronymic(event.currentTarget.value)}
            onBlur={() => commitName('patronymic')}
            onClear={() => {
              setPatronymic('');
              commitField('patronymic', '');
            }}
          />
        </section>
        <PhoneFieldRow phone={formatPhoneDisplay(me.phone)} />
        <TextField
          variant="titleOut"
          title="Электронная почта"
          placeholder="email@example.com"
          value={email}
          onChange={(event) => setEmail(event.currentTarget.value)}
          onBlur={() => commitField('email', email)}
          onClear={() => {
            setEmail('');
            commitField('email', '');
          }}
          error={emailError}
        />
        <TimezoneFieldRow label={formatTimezoneLabel(me.timezone)} />
      </div>
    </div>
  );
}

/** Экран «Аккаунт» в новом дизайне (тикет #593, карта #591; Figma
 * 1789-99036): аватар-плейсхолдер 96 без «Добавить фото» (аватар отложен),
 * поля имени (titleIn) с автосохранением PATCH /me по blur/очистке —
 * макет без кнопки «Сохранить», телефон строкой на /profile/account/phone,
 * почта полем с валидацией, часовой пояс строкой (пикер — #594). Поглотил
 * легаси PersonalDataForm (/profile/personal снесён) и AccountOverview.
 * Тихое автосохранение браузерной таймзоны перенесено из PersonalDataForm
 * как было: один раз, только если зона ещё не сохранена. */
export function AccountScreen(): JSX.Element {
  const { data: me, isError, refetch } = useMe();
  const updateMe = useUpdateMe();
  const autoTzSent = useRef(false);

  // Auto-detect the browser timezone and save it to the profile silently,
  // but only when there is no saved timezone yet (don't override manual
  // settings during trips). Runs once per mount.
  useEffect(() => {
    if (!me || autoTzSent.current) {
      return;
    }
    if (me.timezone) {
      autoTzSent.current = true;
      return;
    }
    if (updateMe.isPending) {
      return;
    }
    const detectedTz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (!detectedTz) {
      autoTzSent.current = true;
      return;
    }
    autoTzSent.current = true;
    updateMe.mutate({ timezone: detectedTz }, {
      onError: (error) => notify.scenarios.profile.personalDataSaveError(error),
    });
  }, [me, updateMe]);

  if (isError) {
    return (
      <div className="flex flex-col items-center gap-4 rounded-3xl bg-surface-muted px-6 py-8">
        <p className="m-0 text-sm text-content-secondary">Не удалось загрузить данные</p>
        <Button variant="white" onClick={() => void refetch()}>
          Повторить
        </Button>
      </div>
    );
  }

  if (me === undefined) {
    return <AccountScreenSkeleton />;
  }

  return <AccountScreenView key={me.id} me={me} />;
}
