'use client';

import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './TenantDetailLoading.module.css';

export function TenantDetailLoading(): JSX.Element {
  return (
    <div className={styles.root} role="status" aria-busy="true" aria-label="Загрузка данных арендатора">
      <Skeleton className={styles.section} />
      <Skeleton className={styles.section} />
    </div>
  );
}
