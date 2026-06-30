import type { JSX } from 'react';
import type { Property } from '@/entities/property/model/types';
import type { components } from '@/shared/api/generated';
import {
  getPropertyPageStatus,
  type PropertyPageStatus,
} from '../lib/get-property-page-status';
import { formatAwaitingStart } from '../lib/format-awaiting-start';
import { formatOverdueCount } from '../lib/format-overdue-count';
import { PropertyDetailStatusBadge } from './PropertyDetailStatusBadge';
import styles from './PropertyStatusSection.module.css';

type LeaseResponse = components['schemas']['LeaseResponse'];
type PropertyOperationsSummaryResponse =
  components['schemas']['PropertyOperationsSummaryResponse'];

export type PropertyStatusSectionProps = {
  readonly property: Property;
  readonly leases: LeaseResponse[];
  readonly summary: PropertyOperationsSummaryResponse | undefined;
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
  leases: LeaseResponse[],
  summary: PropertyOperationsSummaryResponse | undefined,
): string | null {
  if (status === 'awaiting_start') {
    const lease = leases.find((l) => l.status === 'awaiting_start');
    return lease ? formatAwaitingStart(lease.start_date) : null;
  }

  if (status === 'requires_action') {
    const overdue = summary?.overdue_total_count ?? 0;
    if (overdue <= 0) return null;
    return formatOverdueCount(overdue);
  }

  return null;
}
