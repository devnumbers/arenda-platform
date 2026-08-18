'use client';

import type { ComponentType, JSX, SVGProps } from 'react';
import NextLink from 'next/link';
import { ArchiveBold, BadgeDanger, BadgeGood, BadgeInfo, Home } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import type { Operation, OperationStatus } from '@/entities/operation/model/types';
import { getOperationTrailing } from '@/entities/operation/lib/dates';
import { formatMoneyKopecks } from '@/entities/operation/lib/formatMoney';
import styles from './OperationListItem.module.css';

const STATUS_BADGE_ICON: Record<OperationStatus, ComponentType<SVGProps<SVGSVGElement>>> = {
  pending: BadgeInfo,
  overdue: BadgeDanger,
  paid: BadgeGood,
  received: BadgeGood,
  unconfirmed: BadgeInfo,
};

export type OperationListItemProps = {
  readonly operation: Operation;
};

export function OperationListItem({ operation }: OperationListItemProps): JSX.Element {
  const isIncome = operation.type === 'income';
  const sign = isIncome ? '+' : '-';
  const amountClass = isIncome ? styles.amountIncome : styles.amountExpense;
  const trailing = getOperationTrailing(operation);
  const StatusBadgeIcon = STATUS_BADGE_ICON[operation.status];

  return (
    <NextLink href={ROUTES.financeOperation(operation.id)} className={styles.root}>
      <span className={styles.iconCircle}>
        <Home />
        {operation.propertyStatus === 'archived' && (
          <span className={styles.archiveBadge} aria-hidden="true">
            <ArchiveBold />
          </span>
        )}
        {StatusBadgeIcon && <StatusBadgeIcon className={styles.statusBadge} aria-hidden="true" />}
      </span>
      <span className={styles.main}>
        <span className={styles.name}>{operation.name}</span>
      </span>
      <span className={styles.right}>
        <span className={`${styles.amount} ${amountClass}`}>
          {sign}
          {formatMoneyKopecks(operation.amountKopecks, { round: true })}
        </span>
        <span className={styles.trailing}>{trailing}</span>
      </span>
    </NextLink>
  );
}
