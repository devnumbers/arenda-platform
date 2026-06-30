'use client';

import type { JSX, ReactNode } from 'react';
import type { components } from '@/shared/api/generated';
import { OperationListItem } from './OperationListItem';
import { RecurringOperationListItem } from './RecurringOperationListItem';
import styles from './OperationsList.module.css';

type OperationResponse = components['schemas']['OperationResponse'];
type RecurringOperationResponse = components['schemas']['RecurringOperationResponse'];

export type OperationsListItem =
  | { kind: 'onetime'; data: OperationResponse }
  | { kind: 'recurring'; data: RecurringOperationResponse };

export type OperationsListProps = {
  readonly items: ReadonlyArray<OperationsListItem>;
  readonly showProperty?: boolean;
  readonly emptyState?: ReactNode;
};

export function OperationsList({
  items,
  showProperty = false,
  emptyState,
}: OperationsListProps): JSX.Element {
  if (items.length === 0 && emptyState) {
    return <div className={styles.root}>{emptyState}</div>;
  }

  return (
    <ul className={styles.root}>
      {items.map((item) =>
        item.kind === 'onetime' ? (
          <li key={`${item.kind}-${item.data.id}`}>
            <OperationListItem operation={item.data} showProperty={showProperty} />
          </li>
        ) : (
          <RecurringOperationListItem
            key={`${item.kind}-${item.data.id}`}
            operation={item.data}
          />
        ),
      )}
    </ul>
  );
}
