'use client';

import type { JSX } from 'react';
import NextLink from 'next/link';
import { Icon } from '@/shared/ui/icon';
import { Button } from '@/shared/ui/button';
import { LinkButton } from '@/shared/ui/link-button';
import { ArrowRight } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatDuration } from '@/shared/lib/format-duration';
import { formatLeaseMonth } from '@/shared/lib/format-lease-month';
import { LeaseProgressBar } from '@/widgets/properties/ui/LeaseProgressBar';
import type { components } from '@/shared/api/generated';
import type { PropertyPageStatus } from '../lib/get-property-page-status';
import { formatAwaitingStart } from '../lib/format-awaiting-start';
import { formatOverdueCount } from '../lib/format-overdue-count';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyLeaseCard.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];

export type PropertyLeaseCardProps = {
  readonly lease: LeaseResponse | undefined;
  readonly overdueRentCount: number;
  readonly status: PropertyPageStatus;
  readonly propertyId: string;
  readonly onPayRent?: () => void;
};

export function PropertyLeaseCard({
  lease,
  overdueRentCount,
  status,
  propertyId,
  onPayRent,
}: PropertyLeaseCardProps): JSX.Element {
  const leaseNewHref = `${ROUTES.leaseNew}?propertyId=${propertyId}`;
  const endDate = lease?.end_date ?? lease?.start_date;

  return (
    <PropertyDetailSection>
      <div className={styles.header}>
        <h2 className={styles.title}>Аренда</h2>
        <NextLink
          href={ROUTES.finance}
          className={styles.headerLink}
          aria-label="Перейти к аренде"
        >
          <Icon size="s">
            <ArrowRight />
          </Icon>
        </NextLink>
      </div>

      {overdueRentCount > 0 && (
        <span className={styles.overdueBadge}>
          {formatOverdueCount(overdueRentCount)}
        </span>
      )}

      {lease && endDate ? (
        <div className={styles.card}>
          <div className={styles.row}>
            <span className={styles.amount}>
              {formatMoneyKopecks(lease.rent_amount_kopecks)}
            </span>
            <span className={styles.duration}>
              {formatDuration(lease.start_date, endDate)}
            </span>
          </div>

          <LeaseProgressBar
            startDate={lease.start_date}
            endDate={endDate}
            active={status === 'rented' || status === 'requires_action'}
          />

          <div className={styles.row}>
            <span className={styles.tenant}>
              {lease.tenant_contact?.name ?? 'Арендатор не указан'}
            </span>
            <span className={styles.month}>
              {status === 'awaiting_start'
                ? formatAwaitingStart(lease.start_date)
                : formatLeaseMonth(lease.start_date)}
            </span>
          </div>
        </div>
      ) : (
        <div className={styles.empty}>
          <p className={styles.emptyText}>Аренда не создана</p>
        </div>
      )}

      <div className={styles.actions}>
        {status === 'rented' || status === 'requires_action' ? (
          <>
            {onPayRent ? (
              <Button
                variant="primary"
                fullWidth
                onClick={onPayRent}
                type="button"
              >
                Оплатить аренду
              </Button>
            ) : (
              <LinkButton href={ROUTES.finance} variant="primary" fullWidth>
                Оплатить аренду
              </LinkButton>
            )}
            <LinkButton href={ROUTES.finance} variant="secondary" fullWidth>
              Все операции
            </LinkButton>
          </>
        ) : status === 'finished' ? (
          <LinkButton href={leaseNewHref} variant="primary" fullWidth>
            Продлить
          </LinkButton>
        ) : status === 'awaiting_start' ? (
          <LinkButton href={leaseNewHref} variant="primary" fullWidth>
            Начать аренду
          </LinkButton>
        ) : status === 'free' ? (
          <LinkButton href={leaseNewHref} variant="primary" fullWidth>
            Создать аренду
          </LinkButton>
        ) : (
          <span className={styles.disabledText}>
            Аренда недоступна в этом статусе
          </span>
        )}
      </div>
    </PropertyDetailSection>
  );
}
