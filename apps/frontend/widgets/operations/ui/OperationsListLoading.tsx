'use client';

import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import styles from './OperationsListLoading.module.css';

function OperationRowSkeleton(): JSX.Element {
  return (
    <div className={styles.row}>
      <div className={styles.main}>
        <Skeleton className={styles.name} />
        <Skeleton className={styles.meta} />
      </div>
      <div className={styles.right}>
        <Skeleton className={styles.amount} />
        <Skeleton className={styles.status} />
      </div>
    </div>
  );
}

export function OperationsListLoading(): JSX.Element {
  return (
    <div className={styles.root}>
      <OperationRowSkeleton />
      <OperationRowSkeleton />
      <OperationRowSkeleton />
    </div>
  );
}
