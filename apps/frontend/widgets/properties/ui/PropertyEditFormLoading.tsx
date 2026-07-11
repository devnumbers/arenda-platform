import type { JSX } from 'react';
import { Skeleton } from '@heroui/react/skeleton';
import styles from './PropertyEditFormLoading.module.css';

export function PropertyEditFormLoading(): JSX.Element {
  return (
    <div
      className={styles.root}
      aria-busy="true"
      aria-label="Загрузка формы редактирования объекта"
    >
      <div className={styles.header}>
        <Skeleton className={styles.back} />
        <Skeleton className={styles.title} />
      </div>

      <section className={styles.section}>
        <Skeleton className={styles.sectionTitle} />
        <div className={styles.fields}>
          <Skeleton className={styles.field} />
          <Skeleton className={styles.field} />
          <Skeleton className={styles.field} />
          <Skeleton className={styles.multilineField} />
        </div>
      </section>

      <Skeleton className={styles.submit} />
    </div>
  );
}
