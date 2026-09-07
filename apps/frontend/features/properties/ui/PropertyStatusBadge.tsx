import type { JSX } from 'react';
import clsx from 'clsx';
import { BoldWarning, CheckmarkCircle } from '@/shared/assets/icons';
import type { DisplayStatus } from '@/features/properties/lib/property-statuses';
import styles from './PropertyStatusBadge.module.css';

export type PropertyStatusBadgeProps = {
  readonly status: DisplayStatus;
};

type StatusConfig = {
  readonly label?: string;
  readonly color: string;
  readonly icon: React.ComponentType<{ className?: string }>;
};

const config: Record<DisplayStatus, StatusConfig> = {
  // text-error — маркер замены (07.09): канон «по смыслу» до правильных
  // иконок статусов от владельца.
  active: { label: 'Активен', color: '#A1A3A6', icon: CheckmarkCircle },
  maintenance: { label: 'На ремонте', color: '#EBB800', icon: BoldWarning },
};

export function PropertyStatusBadge({ status }: PropertyStatusBadgeProps): JSX.Element {
  const item = config[status];
  const Icon = item.icon;
  const text = item.label;

  return (
    <span className={styles.badge} style={{ color: item.color }}>
      <Icon className={clsx(styles.icon, 'text-error')} />
      {text}
    </span>
  );
}
