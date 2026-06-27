'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './PropertyDetailError.module.css';

export type PropertyDetailErrorProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function PropertyDetailError({
  onRetry,
  isLoading = false,
}: PropertyDetailErrorProps): JSX.Element {
  return (
    <div className={styles.root} role="alert" aria-live="polite">
      <h2 className={styles.title}>Не удалось загрузить объект</h2>
      <p className={styles.subtitle}>Проверьте соединение и попробуйте снова</p>
      <Button
        variant="primary"
        size="medium"
        loading={isLoading}
        onClick={onRetry}
        type="button"
      >
        Повторить
      </Button>
    </div>
  );
}
