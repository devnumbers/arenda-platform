import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './OperationDetailLoading.module.css';

export function OperationDetailLoading(): JSX.Element {
  return (
    <div className={styles.root} aria-busy="true" aria-label="Загрузка операции">
      <div className={styles.card}>
        <div className={styles.header}>
          <Skeleton className={styles.name} />
          <Skeleton className={styles.badge} />
        </div>
        <Skeleton className={styles.amount} />
        <div className={styles.details}>
          <div className={styles.detailRow}>
            <Skeleton className={styles.detailLabel} />
            <Skeleton className={styles.detailValue} />
          </div>
          <div className={styles.detailRow}>
            <Skeleton className={styles.detailLabel} />
            <Skeleton className={styles.detailValue} />
          </div>
          <div className={styles.detailRow}>
            <Skeleton className={styles.detailLabel} />
            <Skeleton className={styles.detailValue} />
          </div>
          <div className={styles.detailRow}>
            <Skeleton className={styles.detailLabel} />
            <Skeleton className={styles.detailValue} />
          </div>
        </div>
      </div>
    </div>
  );
}
