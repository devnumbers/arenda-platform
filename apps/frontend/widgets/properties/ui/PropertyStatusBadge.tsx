import type { JSX } from 'react';
import clsx from 'clsx';
import { getDisplayStatus, displayStatusConfig } from '@/features/properties/lib/property-statuses';
import styles from './PropertyStatusBadge.module.css';

export type PropertyStatusBadgeProps = {
  readonly status: 'active' | 'maintenance' | 'archived';
  readonly occupancy: 'free' | 'occupied';
};

export function PropertyStatusBadge({ status, occupancy }: PropertyStatusBadgeProps): JSX.Element | null {
  const displayStatus = getDisplayStatus(status, occupancy);
  if (!displayStatus) return null;

  const config = displayStatusConfig[displayStatus];

  return (
    <span className={clsx(styles.badge, styles[config.color])}>
      {config.label}
    </span>
  );
}
