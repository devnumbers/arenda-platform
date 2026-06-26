'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './TenantsError.module.css';

export type TenantsErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function TenantsError({ onRetry, isLoading }: TenantsErrorProps): JSX.Element {
  return (
    <div className={styles.root} role="alert" aria-live="polite">
      <p className={styles.text}>Не удалось загрузить арендаторов</p>
      <Button type="button" variant="primary" size="medium" loading={isLoading} onClick={onRetry}>
        Повторить
      </Button>
    </div>
  );
}
