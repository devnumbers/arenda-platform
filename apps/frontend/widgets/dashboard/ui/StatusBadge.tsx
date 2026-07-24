'use client';

import type { JSX } from 'react';
import clsx from 'clsx';
import { Icon } from '@/shared/ui/icon';
import { Good, BadgeInfo } from '@/shared/assets/icons';
import type { components } from '@/shared/api/generated';
import styles from './StatusBadge.module.css';

type LeaseStatus = components['schemas']['LeaseResponse']['status'];

export type StatusBadgeProps = {
  readonly status: LeaseStatus;
};

const statusLabels: Record<LeaseStatus, string> = {
  active: 'Арендован',
  awaiting_start: 'Скоро начнётся',
  requires_action: 'Требует действия',
  completed: 'Арендован',
  archived: 'Арендован',
};

export function StatusBadge({ status }: StatusBadgeProps): JSX.Element {
  return (
    <span className={clsx(styles.root, status === 'awaiting_start' && styles.awaitingStart)}>
      <Icon size="s">
        {status === 'awaiting_start' ? <BadgeInfo /> : <Good />}
      </Icon>
      <span className={styles.label}>{statusLabels[status] ?? statusLabels.active}</span>
    </span>
  );
}
