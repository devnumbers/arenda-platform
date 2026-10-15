import type { JSX } from 'react';
import { PaymentsFavoritesLoading } from '@/widgets/payments';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что у экрана в фазе загрузки; Suspense-границы в page.tsx нет,
 * экран держит pending-гейт сам.
 */
export default function Loading(): JSX.Element {
  return <PaymentsFavoritesLoading />;
}
