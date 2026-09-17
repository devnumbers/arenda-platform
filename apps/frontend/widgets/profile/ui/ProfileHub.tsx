'use client';

import type { ComponentType, JSX } from 'react';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  AccountSetting,
  Exit,
  Info,
  NotificationSettings,
  SmallArrowRight,
  Star,
  Team,
} from '@/shared/assets/icons';
import {
  ConfirmDialog,
  Button,
  Skeleton,
} from '@/shared/ui/design';
import { useLogout, useMe } from '@/features/auth';
import { ROUTES } from '@/shared/config/routes';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { formatPhoneDisplay } from '@/shared/lib/phone';
import { notify } from '@/shared/lib/notifications';
import { getProfileDisplayName } from '../lib/profile-display';
import { AvatarPlaceholder } from './AvatarPlaceholder';

type HubRow = {
  readonly title: string;
  readonly href: string;
  /** Иконка строки — R-стиль 24×24 из канона (DESIGN.md §10). */
  readonly Icon: ComponentType<{ className?: string }>;
};

const navigationRows: readonly HubRow[] = [
  { title: 'Аккаунт', href: ROUTES.profileAccount, Icon: AccountSetting },
  { title: 'Тариф', href: ROUTES.profileTariff, Icon: Star },
  // Хаб «Совместный доступ» (#696): второй вход хаба — строка в профиле
  // (решение чарта #692 №10); позиция после «Тарифа» — на приёмке #696.
  { title: 'Участники', href: ROUTES.participants, Icon: Team },
  { title: 'Уведомления', href: ROUTES.profileNotifications, Icon: NotificationSettings },
  { title: 'Информация', href: ROUTES.profileInfo, Icon: Info },
];

/** Строка меню хаба (Figma «Row Button» 936:39348, Gray-вариант): ведущая
 * иконка 24 — канонный Icon Button Primary (934:19122): прозрачная зона
 * 44 без подложки, глиф сразу на сером контейнере (Figma 1903-38341,
 * правка владельца 10.09.2026); заголовок 16/18, стрелка справа. Строка —
 * div с useKeyboardActivation: иконки декоративны, действие одно на
 * строку. Hover/press приглушают строку (канон §6). */
function HubRowButton({
  title,
  Icon,
  onSelect,
}: {
  readonly title: string;
  readonly Icon: HubRow['Icon'];
  readonly onSelect: () => void;
}): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect });

  return (
    <div
      {...activatorProps}
      className={cn(
        'flex w-full cursor-pointer items-center px-3 py-1 text-left outline-none',
        'transition-opacity hover:opacity-80 active:opacity-80',
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
      )}
    >
      <span aria-hidden className="flex h-11 w-11 shrink-0 items-center justify-center text-content">
        <Icon className="h-6 w-6" />
      </span>
      <span className="min-w-0 flex-1 truncate px-3 text-base font-medium text-content">{title}</span>
      <span
        aria-hidden
        className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
      >
        <SmallArrowRight className="h-6 w-6" />
      </span>
    </div>
  );
}

function ProfileHubSkeleton(): JSX.Element {
  return (
    <div role="status" aria-label="Загрузка профиля" className="flex flex-col gap-8 px-6 pb-6">
      <div className="flex flex-col items-center gap-4">
        <Skeleton className="h-24 w-24 rounded-pill" />
        <div className="flex flex-col items-center gap-2">
          <Skeleton className="h-8 w-40" />
          <Skeleton className="h-4 w-44" />
        </div>
      </div>
      <div className="flex flex-col rounded-3xl bg-surface-muted py-2">
        {navigationRows.map((row) => (
          <div key={row.href} className="flex items-center px-3 py-1">
            <Skeleton className="h-6 w-6 shrink-0 bg-surface-muted-hover" />
            <Skeleton className="ml-[22px] h-5 w-32 flex-1 bg-surface-muted-hover" />
            <Skeleton className="ml-3 h-6 w-6 shrink-0 bg-surface-muted-hover" />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Хаб профиля в новом дизайне (тикет #592, карта #591; Figma 1903-38340
 * плейсхолдер / 1786-31288 заполненный): аватар-плейсхолдер 96 (Bold/User,
 * фото — отложенная карта), имя + телефон, серый контейнер со строками
 * «Аккаунт / Тариф / Участники / Уведомления / Информация / Выйти».
 * Строка «Устройства» скрыта (решение владельца 10.09.2026 — нет макетов
 * и бэка); «Участники» — вход хаба «Совместный доступ» (#696, карта #692).
 * «Выйти» — ConfirmDialog канон + POST /auth/logout. */
export function ProfileHub(): JSX.Element {
  const router = useRouter();
  const { data: me, isError, refetch } = useMe();
  const logout = useLogout();
  const [logoutOpen, setLogoutOpen] = useState(false);

  const handleLogoutConfirm = (): void => {
    logout.mutate(undefined, {
      onSuccess: () => {
        setLogoutOpen(false);
        router.push(ROUTES.login);
      },
      onError: (error) => {
        notify.scenarios.profile.logoutError(error);
      },
    });
  };

  if (isError) {
    return (
      <div className="flex flex-col items-center gap-4 rounded-3xl bg-surface-muted px-6 py-8">
        <p className="text-sm text-content-secondary">Не удалось загрузить профиль</p>
        <Button variant="white" onClick={() => void refetch()}>
          Повторить
        </Button>
      </div>
    );
  }

  if (me === undefined) {
    return <ProfileHubSkeleton />;
  }

  return (
    <div className="flex flex-col gap-8 px-6 pb-6">
      <div className="flex flex-col items-center gap-4">
        <AvatarPlaceholder />
        <div className="flex flex-col items-center gap-2">
          <h1 className="m-0 text-[28px] font-semibold leading-8 text-content">
            {getProfileDisplayName(me)}
          </h1>
          <p className="m-0 text-sm text-content-tertiary">{formatPhoneDisplay(me.phone)}</p>
        </div>
      </div>
      <div className="flex flex-col rounded-3xl bg-surface-muted py-2">
        <nav aria-label="Разделы профиля" className="flex flex-col">
          {navigationRows.map((row) => (
            <HubRowButton
              key={row.href}
              title={row.title}
              Icon={row.Icon}
              onSelect={() => router.push(row.href)}
            />
          ))}
        </nav>
        <HubRowButton title="Выйти" Icon={Exit} onSelect={() => setLogoutOpen(true)} />
      </div>
      <ConfirmDialog
        open={logoutOpen}
        onOpenChange={setLogoutOpen}
        title="Выйти из аккаунта?"
        confirmLabel="Выйти"
        pending={logout.isPending}
        onConfirm={handleLogoutConfirm}
      />
    </div>
  );
}
