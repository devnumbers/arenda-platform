import type { JSX } from 'react';
import { OperationCreateLoading } from '@/widgets/payments';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что у первого шага визарда (форма шага = скелетон, §7 #607);
 * Suspense-границы в page.tsx нет, экран держит pending-гейт сам.
 */
export default function Loading(): JSX.Element {
  return <OperationCreateLoading />;
}
