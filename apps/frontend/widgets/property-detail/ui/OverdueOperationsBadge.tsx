'use client';

import type { JSX } from 'react';
import { Icon } from '@/shared/ui/icon';
import { BadgeDanger } from '@/shared/assets/icons';
import { formatOverdueCount } from '../lib/format-overdue-count';
import styles from './OverdueOperationsBadge.module.css';

export function OverdueOperationsBadge({
  count,
}: {
  readonly count: number;
}): JSX.Element {
  return (
    <span className={styles.badge}>
      <Icon size="s">
        <BadgeDanger />
      </Icon>
      {formatOverdueCount(count)}
    </span>
  );
}
