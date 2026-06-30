'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Icon } from '@/shared/ui/icon';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { ArrowLeft, ArrowRight } from '@/shared/assets/icons';
import { useMe } from '@/features/auth/api/hooks';
import { ROUTES } from '@/shared/config/routes';
import styles from './AccountOverview.module.css';
import {IconLink} from "@/shared/ui/icon-link";

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
        <header className={styles.header}>
            <IconLink
                href={ROUTES.profile}
                aria-label="Назад"
                icon={<ArrowLeft/>}
            />
            <h1 className={styles.title}>Аккаунт</h1>
        </header>
      <Card className={styles.card}>
        <div className={styles.phoneRow}>
          <div>
            <p className={styles.label}>Телефон</p>
            <p className={styles.value}>{me.phone}</p>
          </div>
          <LinkButton
            href={ROUTES.profileChangePhone}
            variant="clear"
            size="small"
            rightIcon={
              <Icon size="s">
                <ArrowRight />
              </Icon>
            }
          >
            Изменить
          </LinkButton>
        </div>
      </Card>
    </section>
  );
}
