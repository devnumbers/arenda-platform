'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './TenantsLoading.module.css';

export function TenantsLoading(): JSX.Element {
  return (
    <div className={styles.root} aria-busy="true" aria-label="Загрузка арендаторов" role="status">
      <Skeleton className={styles.sectionTitle} />
      <Skeleton className={styles.card} />
      <Skeleton className={styles.card} />
      <Skeleton className={styles.sectionTitle} />
      <Skeleton className={styles.card} />
    </div>
  );
}
