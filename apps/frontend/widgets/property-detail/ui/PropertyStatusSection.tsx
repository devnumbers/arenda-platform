import type { JSX } from 'react';
import type { Property, PropertyOperationsSummary } from '@/entities/property';
import type { Lease } from '@/entities/lease';
import {
  getPropertyPageStatus,
  type PropertyPageStatus,
} from '../lib/get-property-page-status';
import { formatAwaitingStart } from '../lib/format-awaiting-start';
import { formatOverdueCount } from '../lib/format-overdue-count';
import { PropertyDetailStatusBadge } from './PropertyDetailStatusBadge';
import styles from './PropertyStatusSection.module.css';

export type PropertyStatusSectionProps = {
  readonly property: Property;
  readonly leases: Lease[];
  readonly summary: PropertyOperationsSummary | undefined;
};

export function PropertyStatusSection({
  property,
  leases,
  summary,
}: PropertyStatusSectionProps): JSX.Element {
  const status = getPropertyPageStatus(property.status, leases);
  const subLabel = getSubLabel(status, leases, summary);

  return (
    <div className={styles.root}>
      <PropertyDetailStatusBadge status={status} text={subLabel ?? undefined} />
      <div className={styles.info}>
        <h2 className={styles.name}>{property.name}</h2>
        <p className={styles.address}>{property.address}</p>
      </div>
    </div>
  );
}

function getSubLabel(
  status: PropertyPageStatus,
  leases: Lease[],
  summary: PropertyOperationsSummary | undefined,
): string | null {
  if (status === 'awaiting_start') {
    const lease = leases.find((l) => l.status === 'awaiting_start');
    return lease ? formatAwaitingStart(lease.startDate) : null;
  }

  if (status === 'requires_action') {
    const overdue = summary?.overdueTotalCount ?? 0;
    if (overdue <= 0) return null;
    return formatOverdueCount(overdue);
  }

  return null;
}
