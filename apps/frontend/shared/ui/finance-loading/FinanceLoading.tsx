'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './FinanceLoading.module.css';

function FinanceCardSkeleton(): JSX.Element {
  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <div className={styles.meta}>
          <Skeleton className={styles.title} />
          <Skeleton className={styles.badge} />
        </div>
        <Skeleton className={styles.amount} />
      </div>
      <div className={styles.body}>
        <div className={styles.row}>
          <Skeleton className={styles.shortBar} />
          <Skeleton className={styles.shortBar} />
        </div>
        <div className={styles.row}>
          <Skeleton className={styles.longBar} />
          <Skeleton className={styles.shortBar} />
        </div>
      </div>
    </div>
  );
}

export function FinanceLoading(): JSX.Element {
  return (
    <div className={styles.root}>
      <FinanceCardSkeleton />
      <FinanceCardSkeleton />
      <FinanceCardSkeleton />
    </div>
  );
}
