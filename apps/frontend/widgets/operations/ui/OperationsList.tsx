'use client';

import type { JSX } from 'react';
import type { components } from '@/shared/api/generated';
import { OperationListItem } from './OperationListItem';
import styles from './OperationsList.module.css';

type OperationResponse = components['schemas']['OperationResponse'];

export type OperationsListProps = {
  readonly items: ReadonlyArray<OperationResponse>;
  readonly showProperty?: boolean;
};

export function OperationsList({ items, showProperty = false }: OperationsListProps): JSX.Element {
  return (
    <ul className={styles.root}>
      {items.map((operation) => (
        <li key={operation.id}>
          <OperationListItem operation={operation} showProperty={showProperty} />
        </li>
      ))}
    </ul>
  );
}
