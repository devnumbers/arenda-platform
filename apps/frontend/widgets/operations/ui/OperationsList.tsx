'use client';

import type { JSX, ReactNode } from 'react';
import type { components } from '@/shared/api/generated';
import { OperationListItem } from './OperationListItem';
import styles from './OperationsList.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type OperationsListProps = {
  readonly operations: ReadonlyArray<OperationResponse>;
  readonly emptyState?: ReactNode;
};

export function OperationsList({ operations, emptyState }: OperationsListProps): JSX.Element {
  if (operations.length === 0 && emptyState) {
    return <div className={styles.root}>{emptyState}</div>;
  }

  return (
    <ul className={styles.root}>
      {operations.map((operation) => (
        <li key={operation.id}>
          <OperationListItem operation={operation} />
        </li>
      ))}
    </ul>
  );
}
