'use client';

import type { JSX } from 'react';
import { Button } from '@/shared/ui/design';
import styles from './PropertiesErrorState.module.css';

export type PropertiesErrorStateProps = {
  readonly onRetry: () => void;
  readonly isLoading?: boolean;
};

export function PropertiesErrorState({ onRetry, isLoading = false }: PropertiesErrorStateProps): JSX.Element {
  return (
    <div className={styles.root} role="alert" aria-live="polite">
      <h2 className={styles.title}>Не удалось загрузить объекты</h2>
      <p className={styles.subtitle}>Проверьте соединение и попробуйте снова</p>
      {/* Канонный Button (легаси снесён, #901): type явный — канон,
          в отличие от легаси, не дефолтит type="button". */}
      <Button loading={isLoading} onClick={onRetry} type="button">
        Повторить
      </Button>
    </div>
  );
}
