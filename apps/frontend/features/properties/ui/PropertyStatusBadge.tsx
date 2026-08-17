import type { JSX } from 'react';
import { StatusGood, StatusWarning, StatusDanger, StatusDoor, BadgeDanger, BadgeInfo } from '@/shared/assets/icons';
import type { DisplayStatus } from '@/features/properties/lib/property-statuses';
import { formatOverduePaymentsCount } from '@/features/properties/lib/property-statuses';
import styles from './PropertyStatusBadge.module.css';

export type PropertyStatusBadgeProps = {
  readonly status: DisplayStatus;
  readonly overdueCount?: number;
};

type StatusConfig = {
  readonly label?: string;
  readonly color: string;
  readonly icon: React.ComponentType<{ className?: string }>;
};

const config: Record<DisplayStatus, StatusConfig> = {
  rented: { label: 'Арендован', color: '#34C771', icon: StatusGood },
  requires_action: { label: 'Требует действия', color: '#FF4646', icon: StatusDanger },
  awaiting_start: { label: 'Скоро начнётся', color: '#2B7FFF', icon: BadgeInfo },
  free: { label: 'Не арендован', color: '#A1A3A6', icon: StatusDoor },
  maintenance: { label: 'На ремонте', color: '#EBB800', icon: StatusWarning },
  overdue: { color: '#FF4646', icon: BadgeDanger },
};

export function PropertyStatusBadge({ status, overdueCount }: PropertyStatusBadgeProps): JSX.Element {
  const item = config[status];
  const Icon = item.icon;
  const text = status === 'overdue' ? formatOverduePaymentsCount(overdueCount ?? 0) : item.label;

  return (
    <span className={styles.badge} style={{ color: item.color }}>
      <Icon className={styles.icon} />
      {text}
    </span>
  );
}
