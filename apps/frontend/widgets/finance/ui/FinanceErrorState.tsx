'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './FinanceErrorState.module.css';

export type FinanceErrorStateProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function FinanceErrorState({ onRetry, isLoading = false }: FinanceErrorStateProps): JSX.Element {
  return (
    <div className={styles.root} role="alert" aria-live="polite">
      <h2 className={styles.title}>Не удалось загрузить финансовые данные</h2>
      <p className={styles.subtitle}>Проверьте соединение и попробуйте снова</p>
      <Button variant="primary" size="medium" loading={isLoading} onClick={onRetry}>
        Повторить
      </Button>
    </div>
  );
}
