import type { JSX } from 'react';
import { NotificationsLoading } from '@/widgets/notifications';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что и fallback Suspense-границы страницы.
 */
export default function Loading(): JSX.Element {
  return <NotificationsLoading />;
}
