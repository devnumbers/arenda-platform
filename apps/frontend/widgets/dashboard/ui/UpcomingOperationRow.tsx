'use client';

import type { JSX } from 'react';
import { OperationListItem } from '@/widgets/operations/ui/OperationListItem';
import type { OperationListItemProps } from '@/widgets/operations/ui/OperationListItem';

export type UpcomingOperationRowProps = Omit<OperationListItemProps, 'variant'>;

export function UpcomingOperationRow({ operation }: UpcomingOperationRowProps): JSX.Element {
  return <OperationListItem operation={operation} variant="dashboard" />;
}
