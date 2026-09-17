'use client';

import { useCallback, useState, type JSX } from 'react';
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
import { profileFieldPatch, type ProfileTextField } from '../lib/profile-edit';
import { AvatarPlaceholder } from './AvatarPlaceholder';

/** Серый бокс поля (как у канонных TextField/PickerField): строка-поле
 * «Телефон», «Электронная почта» и «Часовой пояс» экрана (Figma 1789-99036 —
 * Input Field 1218:53539 Title Out с иконкой в хвосте). */
const fieldBoxClass = cn(
  'flex h-14 w-full items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2',
);

function FieldTitle({ children }: { readonly children: string }): JSX.Element {
  return (
    <span className="text-base font-medium leading-[18px] text-content">{children}</span>
  );
}

/** Хвост-иконка строки-поля (зона 44): тёмный глиф text-content — как у
 * «Телефона» (решение владельца 11.09.2026 — только цвет, форма и зона
 * прежние). */
function FieldTailIcon({ children }: { readonly children: JSX.Element }): JSX.Element {
  return (
    <span
      aria-hidden
      className="flex h-11 w-11 shrink-0 items-center justify-center text-content"
    >
      {children}
    </span>
  );
}

/** Строка-поле, ведущая на флоу смены: «Телефон» (Figma 1789-99037,
 * переработка #595) — на /profile/account/phone, «Электронная почта»
 * (Figma 1789-99036, тикет #722) — на подтверждаемый флоу
 * /profile/account/email (карта #723; инлайн-редактирование снесено:
 * PATCH /me почту больше не принимает, #720 Q1). Значение +
 * SmallArrowRight; хвост без зоны 44 — тёмный глиф text-content, прижат
 * к краю бокса (решение владельца 10.09.2026). Пустое значение — серым
 * «Не указана». */
function NavFieldRow({
  title,
  href,
  value,
}: {
  readonly title: string;
  readonly href: string;
  readonly value: string | null;
}): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <FieldTitle>{title}</FieldTitle>
      <NextLink
        href={href}
        className={cn(
          fieldBoxClass,
          'cursor-pointer transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
        )}
      >
        <span
          className={cn(
            'min-w-0 flex-1 truncate text-base leading-[18px]',
            value === null ? 'text-content-secondary' : 'text-content',
          )}
        >
          {value ?? 'Не указана'}
        </span>
        <span
          aria-hidden
          className="flex shrink-0 items-center justify-center text-content"
        >
          <SmallArrowRight className="h-6 w-6" />
        </span>
      </NextLink>
    </div>
  );
}

/** Строка «Часовой пояс» (Figma 1789-99036): значение + SmallArrowDown.
 * Вся строка — одна ссылка на пикер часового пояса (#594, Figma
 * 1869-70821); глиф тёмный, как у «Телефона» (решение владельца
 * 11.09.2026 — только цвет, зона 44 и форма прежние). */
function TimezoneFieldRow({ label }: { readonly label: string }): JSX.Element {
  return (
    <div className="flex flex-col gap-2">
      <FieldTitle>Часовой пояс</FieldTitle>
      <NextLink
        href={ROUTES.profileAccountTimezone}
        className={cn(
          fieldBoxClass,
          'cursor-pointer outline-none transition-shadow hover:shadow-[inset_0_0_0_2px_var(--dl-input-border)]',
          'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2',
        )}
      >
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
      </NextLink>
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

  /** Тихое автосохранение (макет без кнопки «Сохранить»): поле коммитится
   * на blur и крестиком очистки — патч только этого поля; пустая строка
   * очищает значение (контракт бэка: null = «не менять»). Почта здесь не
   * правится (#722) — только через флоу смены на
   * /profile/account/email. */
  const commitField = useCallback(
    (field: ProfileTextField, rawValue: string) => {
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
        <NavFieldRow
          title="Телефон"
          href={ROUTES.profileChangePhone}
          value={formatPhoneDisplay(me.phone)}
        />
        <NavFieldRow
          title="Электронная почта"
          href={ROUTES.profileChangeEmail}
          value={me.email}
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
 * почта строкой на флоу смены /profile/account/email (#722 — инлайн-правка
 * снесена, PATCH /me почту не принимает), часовой пояс строкой на пикер
 * /profile/account/timezone (#594). Поглотил
 * легаси PersonalDataForm (/profile/personal снесён) и AccountOverview.
 * Тихий автосейв браузерной зоны снесён (#451): колонка users.timezone
 * NOT NULL DEFAULT 'Europe/Moscow' делала условие «если пусто» всегда
 * ложным, а зона устройства теперь фиксируется один раз — при регистрации. */
export function AccountScreen(): JSX.Element {
  const { data: me, isError, refetch } = useMe();

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
