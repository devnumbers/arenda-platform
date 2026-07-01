'use client';

import type { JSX, ComponentType } from 'react';
import {
  StatusGood,
  StatusWarning,
  StatusDanger,
  StatusInfo,
  StatusDoor,
} from '@/shared/assets/icons';
import type { PropertyPageStatus } from '../lib/get-property-page-status';
import styles from './PropertyDetailStatusBadge.module.css';

const config: Record<
  PropertyPageStatus,
  {
    label: string;
    color: string;
    icon: ComponentType<{ className?: string }>;
  }
> = {
  rented: { label: 'Арендована', color: '#34C771', icon: StatusGood },
  requires_action: {
    label: 'Требует действия',
    color: '#FF4646',
    icon: StatusDanger,
  },
  awaiting_start: {
    label: 'Аренда скоро начнётся',
    color: '#2B7FFF',
    icon: StatusInfo,
  },
  free: { label: 'Не арендована', color: '#A1A3A6', icon: StatusDoor },
  maintenance: { label: 'На ремонте', color: '#EBB800', icon: StatusWarning },
  archived: { label: 'В архиве', color: '#A1A3A6', icon: StatusDoor },
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
    <span className={styles.badge} style={{ color: item.color }}>
      <Icon className={styles.icon} aria-hidden="true" />
      {text ?? item.label}
    </span>
  );
}
