'use client';

import type { JSX, ComponentType } from 'react';
import {
  ArchiveBold,
  BadgeDanger,
  BadgeGood,
  BadgeInfo,
  StatusDoor,
  StatusWarning,
} from '@/shared/assets/icons';
import type { PropertyPageStatus } from '../lib/get-property-page-status';
import styles from './PropertyDetailStatusBadge.module.css';

const config: Record<
  PropertyPageStatus,
  {
    label: string;
    className: string;
    icon: ComponentType<{ className?: string }>;
  }
> = {
  rented: { label: 'Арендована', className: styles.rented, icon: BadgeGood },
  requires_action: {
    label: 'Требует действия',
    className: styles.requiresAction,
    icon: BadgeDanger,
  },
  awaiting_start: {
    label: 'Аренда скоро начнётся',
    className: styles.awaitingStart,
    icon: BadgeInfo,
  },
  free: { label: 'Не арендована', className: styles.free, icon: StatusDoor },
  maintenance: {
    label: 'На ремонте',
    className: styles.maintenance,
    icon: StatusWarning,
  },
  archived: { label: 'В архиве', className: styles.archived, icon: ArchiveBold },
};

export function PropertyDetailStatusBadge({
  status,
  text,
}: {
  readonly status: PropertyPageStatus;
  readonly text?: string;
}): JSX.Element {
  const item = config[status];
  const Icon = item.icon;
  return (
    <span className={`${styles.badge} ${item.className}`}>
      <Icon className={styles.icon} aria-hidden="true" />
      {text ?? item.label}
    </span>
  );
}
