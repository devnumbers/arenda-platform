'use client';

import type { JSX } from 'react';
import { getDisplayStatus, displayStatusConfig } from '@/features/properties/lib/property-statuses';
import type { components } from '@/shared/api/generated';
import styles from './PropertyStatusBadge.module.css';

type PropertyResponse = components['schemas']['PropertyResponse'];

export type PropertyStatusBadgeProps = {
  readonly property: PropertyResponse;
};

export function PropertyStatusBadge({ property }: PropertyStatusBadgeProps): JSX.Element | null {
  const displayStatus = getDisplayStatus(property.status, property.occupancy);
  if (!displayStatus) return null;

  const config = displayStatusConfig[displayStatus];

  return (
    <span className={`${styles.badge} ${styles[config.color]}`}>
      {config.label}
    </span>
  );
}
