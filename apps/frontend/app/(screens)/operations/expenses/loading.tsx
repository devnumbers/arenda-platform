import type { JSX } from 'react';
import { OperationsDirectionLoading } from '@/widgets/payments';

/**
 * Route-loading направления «Расходы» (#609).
 */
export default function Loading(): JSX.Element {
  return <OperationsDirectionLoading title="Расходы" />;
}
