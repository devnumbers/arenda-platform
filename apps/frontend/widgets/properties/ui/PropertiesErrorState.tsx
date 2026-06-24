'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/button';
import styles from './PropertiesErrorState.module.css';

export type PropertiesErrorStateProps = {
  readonly onRetry: () => void;
};

export function PropertiesErrorState({ onRetry }: PropertiesErrorStateProps): JSX.Element {
  return (
    <div className={styles.root}>
      <h3 className={styles.title}>Не удалось загрузить объекты</h3>
      <p className={styles.subtitle}>Проверьте соединение и попробуйте снова</p>
      <Button variant="primary" size="medium" onClick={onRetry}>
        Повторить
      </Button>
    </div>
  );
}
