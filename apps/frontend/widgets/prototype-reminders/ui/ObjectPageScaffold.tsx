// PROTOTYPE — throwaway, issue #105

import type { JSX, ReactNode } from 'react';
import styles from './ObjectPageScaffold.module.css';

export type ObjectPageScaffoldProps = {
  readonly children: ReactNode;
};

// Статичный каркас страницы объекта: заголовок и блоки-заглушки
// нужны только как визуальный контекст для блока «Напоминания».
export function ObjectPageScaffold({ children }: ObjectPageScaffoldProps): JSX.Element {
  return (
    <div className={styles.root}>
      <h1 className={styles.title}>Объект: 2-к квартира, Ленина 15</h1>
      <div className={styles.placeholder} aria-hidden="true">
        <span className={styles.placeholderTitle}>Основная информация</span>
        <span className={styles.placeholderText}>Заглушка для визуального контекста</span>
      </div>
      {children}
      <div className={styles.placeholder} aria-hidden="true">
        <span className={styles.placeholderTitle}>Операции по объекту</span>
        <span className={styles.placeholderText}>Заглушка для визуального контекста</span>
      </div>
    </div>
  );
}
