'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './PropertiesLoading.module.css';

export function PropertiesLoading(): JSX.Element {
  return (
    <div className={styles.root}>
      <Skeleton className={styles.card} />
      <Skeleton className={styles.card} />
      <Skeleton className={styles.card} />
    </div>
  );
}
