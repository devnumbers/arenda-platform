import type { JSX } from 'react';
import type { Property } from '@/entities/property';
import { getPropertyPageStatus } from '../lib/get-property-page-status';
import { PropertyDetailStatusBadge } from './PropertyDetailStatusBadge';
import styles from './PropertyStatusSection.module.css';

export type PropertyStatusSectionProps = {
  readonly property: Property;
};

export function PropertyStatusSection({
  property,
}: PropertyStatusSectionProps): JSX.Element {
  const status = getPropertyPageStatus(property.status);

  return (
    <div className={styles.root}>
      <PropertyDetailStatusBadge status={status} />
      <div className={styles.info}>
        <h2 className={styles.name}>{property.name}</h2>
        <p className={styles.address}>{property.address}</p>
      </div>
    </div>
  );
}
