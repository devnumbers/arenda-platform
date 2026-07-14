'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './PropertySuccessBanner.module.css';

export type PropertySuccessBannerProps = {
  readonly onOpenLease: () => void;
};

export function PropertySuccessBanner({
  onOpenLease,
}: PropertySuccessBannerProps): JSX.Element {
  return (
    <div className={styles.root}>
      <span className={styles.text}>Аренда завершена</span>
      <div className={styles.actions}>
        <Button variant="clear" size="small" onClick={onOpenLease} type="button">
          Открыть аренду
        </Button>
      </div>
    </div>
  );
}
