'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './TenantDetailError.module.css';

export type TenantDetailErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function TenantDetailError({ onRetry, isLoading }: TenantDetailErrorProps): JSX.Element {
  return (
    <div className={styles.root} role="alert" aria-live="polite">
      <p className={styles.text}>Не удалось загрузить данные арендатора</p>
      <Button
        type="button"
        variant="primary"
        size="medium"
        loading={isLoading}
        onClick={onRetry}
      >
        Повторить
      </Button>
    </div>
  );
}
