'use client';

import type { JSX } from 'react';
import { Card } from '@heroui/react/card';
import { Skeleton } from '@heroui/react/skeleton';
import { Icon } from '@/shared/ui/icon';
import { Clock, BoldWallet, ArrowRight } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import type { Property } from '@/entities/property/model/types';
import { formatDeadline } from '../lib/deadline-helpers';
import { EmptyState } from '@/shared/ui/empty-state';
import { SectionHeader } from './SectionHeader';
import { PaymentRow } from './PaymentRow';
import { IconActionCard } from './IconActionCard';
import styles from './PaymentsSection.module.css';

type ReminderResponse = components['schemas']['ReminderResponse'];

type PaymentsSectionProps = {
  readonly reminders: ReminderResponse[] | undefined;
  readonly properties?: Property[] | undefined;
  readonly isLoading: boolean;
};

function getPropertyName(
  propertyId: string | null | undefined,
  properties: Property[] | undefined,
): string | undefined {
  if (!propertyId || !properties) {
    return undefined;
  }

  return properties.find((property) => property.id === propertyId)?.name;
}

export function PaymentsSection({ reminders, properties, isLoading }: PaymentsSectionProps): JSX.Element {
  const items = reminders?.slice(0, 3) ?? [];

  if (isLoading) {
    return (
      <section className={styles.section}>
        <Skeleton className={styles.titleSkeleton} />
        <Card className={styles.card}>
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.rowSkeleton} />
          <Skeleton className={styles.rowSkeleton} />
          <div className={styles.actionsSkeleton}>
            <Skeleton className={styles.actionSkeleton} />
            <Skeleton className={styles.actionSkeleton} />
            <Skeleton className={styles.actionSkeleton} />
          </div>
        </Card>
      </section>
    );
  }

  if (items.length === 0) {
    return (
      <section className={styles.section}>
        <SectionHeader title="Ближайшие платежи" href="/finance" />
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
      <SectionHeader title="Ближайшие платежи" href="/finance" />
      <Card className={styles.card}>
        <ul className={styles.list}>
          {items.map((reminder) => (
            <li key={reminder.id}>
              <PaymentRow
                icon={
                  <Icon size="m">
                    <Clock />
                  </Icon>
                }
                title={reminder.message_title}
                subtitle={getPropertyName(reminder.property_id, properties) ?? reminder.message_body}
                amount=""
                deadline={formatDeadline(reminder.scheduled_at)}
              />
            </li>
          ))}
        </ul>
        <div className={styles.actions}>
          <IconActionCard
            href="/finance"
            icon={
              <Icon size="l">
                <BoldWallet />
              </Icon>
            }
            label="Внести платеж"
            variant="filled"
          />
          <IconActionCard
            href="/finance"
            icon={
              <Icon size="l">
                <Clock />
              </Icon>
            }
            label="Запланировать"
            variant="filled"
          />
          <IconActionCard
            href="/finance"
            icon={
              <Icon size="l">
                <ArrowRight />
              </Icon>
            }
            label="Все платежи"
            variant="outlined"
          />
        </div>
      </Card>
    </section>
  );
}
