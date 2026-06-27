'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { toast } from 'react-toastify';
import { IconLink } from '@/shared/ui/icon-link';
import { Button } from '@/shared/ui/button';
import { Icon } from '@/shared/ui/icon';
import { ArrowLeft, StarColored } from '@/shared/assets/icons';
import { useMe, useLogout } from '@/features/auth/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import type { User } from '@/entities/user/model/types';
import { getTariffLabel } from '@/entities/user/lib/get-tariff-label';
import { ProfileMenu } from './ProfileMenu';
import styles from './ProfileOverview.module.css';

function getFullName(user: User): string {
  const parts = [user.surname, user.name, user.patronymic].filter(Boolean);
  return parts.join(' ') || user.phone;
}

function UserCardSkeleton(): JSX.Element {
  return (
    <Card className={styles.userCard}>
      <Skeleton className={styles.nameSkeleton} />
      <Skeleton className={styles.fieldSkeleton} />
      <Skeleton className={styles.fieldSkeleton} />
    </Card>
  );
}

export function ProfileOverview(): JSX.Element {
  const router = useRouter();
  const { data: me, isError, refetch } = useMe();
  const logout = useLogout();

  const handleLogout = () => {
    logout.mutate(undefined, {
      onSuccess: () => {
        router.push(ROUTES.login);
      },
      onError: (error) => {
        toast.error(error.detail);
      },
    });
  };

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <IconLink
          href={ROUTES.dashboard}
          aria-label="Назад"
          icon={<ArrowLeft />}
        />
        <h1 className={styles.title}>Профиль</h1>
      </header>

      {isError ? (
        <div className={styles.error}>
          <p className={styles.errorText}>Не удалось загрузить профиль</p>
          <Button onClick={() => refetch()} variant="secondary">
            Повторить
          </Button>
        </div>
      ) : !me ? (
        <UserCardSkeleton />
      ) : (
        <Card className={styles.userCard}>
          <div className={styles.userHeader}>
            <span className={styles.userName}>{getFullName(me)}</span>
            <span className={styles.tariffBadge}>
              <Icon size="s">
                <StarColored />
              </Icon>
              <span>{getTariffLabel(me.subscription?.tariff?.name)}</span>
            </span>
          </div>
          <p className={styles.userField}>{me.phone}</p>
          {me.email && <p className={styles.userField}>{me.email}</p>}
        </Card>
      )}

      <ProfileMenu />

      <Button
        variant="secondary"
        fullWidth
        onClick={handleLogout}
        loading={logout.isPending}
      >
        Выйти
      </Button>
    </div>
  );
}
