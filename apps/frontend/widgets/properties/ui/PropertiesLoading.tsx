'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './PropertiesLoading.module.css';

function PropertyCardSkeleton(): JSX.Element {
  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <div className={styles.meta}>
          <Skeleton className={styles.title} />
          <Skeleton className={styles.badge} />
        </div>
        <Skeleton className={styles.thumbnail} />
      </div>
      <div className={styles.body}>
        <div className={styles.row}>
          <Skeleton className={styles.shortBar} />
          <Skeleton className={styles.shortBar} />
        </div>
        <div className={styles.progress}>
          <Skeleton className={styles.segment} />
          <Skeleton className={styles.segment} />
          <Skeleton className={styles.segment} />
          <Skeleton className={styles.segment} />
        </div>
        <div className={styles.row}>
          <Skeleton className={styles.shortBar} />
          <Skeleton className={styles.shortBar} />
        </div>
      </div>
    </div>
  );
}

export function PropertiesLoading(): JSX.Element {
  return (
    <div className={styles.root}>
      <PropertyCardSkeleton />
      <PropertyCardSkeleton />
      <PropertyCardSkeleton />
    </div>
  );
}
