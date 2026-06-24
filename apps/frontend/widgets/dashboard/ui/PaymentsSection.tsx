'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { LinkButton } from '@/shared/ui/link-button';
import { Icon } from '@/shared/ui/icon';
import { Clock } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import { EmptyState } from './EmptyState';
import styles from './PaymentsSection.module.css';

type ReminderResponse = components['schemas']['ReminderResponse'];

type PaymentsSectionProps = {
  readonly reminders: ReminderResponse[] | undefined;
  readonly isLoading: boolean;
};

function formatDate(iso: string): string {
  return new Intl.DateTimeFormat('ru-RU', {
    day: 'numeric',
    month: 'long',
  }).format(new Date(iso));
}

export function PaymentsSection({
  reminders,
  isLoading,
}: PaymentsSectionProps): JSX.Element {
  const items = reminders?.slice(0, 3) ?? [];

  if (isLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <Card className={styles.card}>
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.actionsSkeleton} />
        </Card>
      </section>
    );
  }

  if (items.length === 0) {
    return (
      <section className={styles.section}>
        <h2 className={styles.title}>Ближайшие платежи</h2>
        <EmptyState
          icon={<Clock />}
          entities="платежей"
          subtitle="Запланируйте платежи, чтобы видеть их здесь"
          actionHref="/finance"
          actionText="Все платежи"
        />
      </section>
    );
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>Ближайшие платежи</h2>
      <Card className={styles.card}>
        <ul className={styles.list}>
          {items.map((reminder) => (
            <li key={reminder.id} className={styles.item}>
              <Icon size="m">
                <Clock />
              </Icon>
              <div className={styles.info}>
                <span className={styles.itemTitle}>{reminder.message_title}</span>
                <span className={styles.itemDate}>
                  {formatDate(reminder.scheduled_at)}
                </span>
              </div>
            </li>
          ))}
        </ul>
        <div className={styles.actions}>
          <LinkButton href="/finance" variant="primary" size="medium" fullWidth>
            Внести платеж
          </LinkButton>
          <LinkButton href="/finance" variant="secondary" size="medium" fullWidth>
            Запланировать
          </LinkButton>
        </div>
        <LinkButton
          href="/finance"
          variant="clear"
          size="medium"
          fullWidth
        >
          Все платежи
        </LinkButton>
      </Card>
    </section>
  );
}
