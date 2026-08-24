'use client';

import clsx from 'clsx';
import type { JSX, ComponentType } from 'react';
import {
  ArchiveBold,
  StatusDoor,
  StatusWarning,
} from '@/shared/assets/icons';
import type { PropertyStatus } from '@/entities/property';
import styles from './PropertyDetailStatusBadge.module.css';

const config: Record<
  PropertyStatus,
  {
    label: string;
    className: string;
    icon: ComponentType<{ className?: string }>;
  }
> = {
  active: { label: 'Активен', className: styles.active ?? '', icon: StatusDoor },
  maintenance: {
    label: 'На ремонте',
    className: styles.maintenance ?? '',
    icon: StatusWarning,
  },
  archived: { label: 'В архиве', className: styles.archived ?? '', icon: ArchiveBold },
};

export function PropertyDetailStatusBadge({
  status,
  text,
}: {
  readonly status: PropertyStatus;
  readonly text?: string;
}): JSX.Element {
  const item = config[status];
  const Icon = item.icon;
  return (
    <span className={clsx(styles.badge, item.className)}>
      <Icon className={styles.icon} aria-hidden="true" />
      {text ?? item.label}
    </span>
  );
}
