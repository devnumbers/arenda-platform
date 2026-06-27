'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Card } from '@heroui/react/card';
import { Icon } from '@/shared/ui/icon';
import { Button } from '@/shared/ui/button';
import { ArrowRight } from '@/shared/assets/icons';
import { useMe } from '@/features/auth/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import styles from './AccountOverview.module.css';

function AccountOverviewSkeleton(): JSX.Element {
  return (
    <div className={styles.section}>
      <div className={styles.skeletonRow} />
    </div>
  );
}

export function AccountOverview(): JSX.Element {
  const { data: me, isPending, isError, refetch } = useMe();

  if (isError) {
    return (
      <div className={styles.error}>
        <p className={styles.errorText}>Не удалось загрузить данные</p>
        <Button onClick={() => refetch()} variant="secondary">
          Повторить
        </Button>
      </div>
    );
  }

  if (isPending || !me) {
    return <AccountOverviewSkeleton />;
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>Аккаунт</h2>
      <Card className={styles.card}>
        <div className={styles.phoneRow}>
          <div>
            <p className={styles.label}>Телефон</p>
            <p className={styles.value}>{me.phone}</p>
          </div>
          <NextLink href={ROUTES.profileChangePhone} className={styles.link}>
            <span className={styles.linkText}>Изменить</span>
            <Icon size="s">
              <ArrowRight />
            </Icon>
          </NextLink>
        </div>
      </Card>
    </section>
  );
}
