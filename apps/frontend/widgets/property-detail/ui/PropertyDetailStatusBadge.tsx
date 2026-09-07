'use client';

import clsx from 'clsx';
import type { JSX, ComponentType } from 'react';
import {
  BoldArchive,
  BoldWarning,
  CheckmarkCircle,
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
  // text-error — маркер замены (07.09): канон «по смыслу» до правильных
  // иконок статусов от владельца.
  active: {
    label: 'Активен',
    className: clsx(styles.active ?? '', 'text-error'),
    icon: CheckmarkCircle,
  },
  maintenance: {
    label: 'На ремонте',
    className: clsx(styles.maintenance ?? '', 'text-error'),
    icon: BoldWarning,
  },
  archived: { label: 'В архиве', className: styles.archived ?? '', icon: BoldArchive },
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
