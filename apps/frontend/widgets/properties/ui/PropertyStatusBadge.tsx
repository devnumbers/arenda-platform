import type { JSX } from 'react';
import clsx from 'clsx';
import { getDisplayStatus, displayStatusConfig } from '@/features/properties/lib/property-statuses';
import type { PropertyStatus, Occupancy } from '@/entities/property/model/types';
import styles from './PropertyStatusBadge.module.css';

export type PropertyStatusBadgeProps = {
  readonly status: PropertyStatus;
  readonly occupancy: Occupancy;
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
