import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './PropertyDetailLoading.module.css';

export function PropertyDetailLoading(): JSX.Element {
  return (
    <div className={styles.root} aria-busy="true" aria-label="Загрузка объекта">
      <div className={styles.header}>
        <Skeleton className={styles.title} />
        <Skeleton className={styles.icon} />
      </div>

      <Skeleton className={styles.gallery} />

      <div className={styles.status}>
        <Skeleton className={styles.badge} />
        <Skeleton className={styles.name} />
        <Skeleton className={styles.address} />
      </div>

      <div className={styles.section}>
        <Skeleton className={styles.sectionTitle} />
        <div className={styles.card}>
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
            <Skeleton className={styles.longBar} />
            <Skeleton className={styles.shortBar} />
          </div>
        </div>
        <div className={styles.actions}>
          <Skeleton className={styles.action} />
          <Skeleton className={styles.action} />
        </div>
      </div>

      <div className={styles.section}>
        <Skeleton className={styles.sectionTitle} />
        <div className={styles.card}>
          <div className={styles.row}>
            <Skeleton className={styles.icon} />
            <Skeleton className={styles.longBar} />
          </div>
          <Skeleton className={styles.longBar} />
        </div>
      </div>

      <div className={styles.section}>
        <Skeleton className={styles.sectionTitle} />
        <div className={styles.card}>
          <div className={styles.row}>
            <Skeleton className={styles.longBar} />
            <Skeleton className={styles.shortBar} />
          </div>
          <div className={styles.row}>
            <Skeleton className={styles.longBar} />
            <Skeleton className={styles.shortBar} />
          </div>
          <div className={styles.row}>
            <Skeleton className={styles.longBar} />
            <Skeleton className={styles.shortBar} />
          </div>
        </div>
        <div className={styles.actions}>
          <Skeleton className={styles.action} />
          <Skeleton className={styles.action} />
        </div>
      </div>

      <div className={styles.section}>
        <Skeleton className={styles.sectionTitle} />
        <div className={styles.card}>
          <div className={styles.row}>
            <Skeleton className={styles.shortBar} />
            <Skeleton className={styles.shortBar} />
          </div>
          <div className={styles.row}>
            <Skeleton className={styles.shortBar} />
            <Skeleton className={styles.shortBar} />
          </div>
        </div>
      </div>

      <div className={styles.section}>
        <Skeleton className={styles.sectionTitle} />
        <Skeleton className={styles.card} />
      </div>
    </div>
  );
}
