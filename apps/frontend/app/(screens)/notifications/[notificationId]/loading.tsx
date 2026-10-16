import type { JSX } from 'react';
import { NotificationDetailLoading } from '@/widgets/notifications';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что и фаза загрузки экрана.
 */
export default function Loading(): JSX.Element {
  return <NotificationDetailLoading />;
}
