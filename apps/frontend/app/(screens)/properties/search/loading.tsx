import type { JSX } from 'react';
import { PropertiesSearchLoading } from '@/widgets/properties';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же кадр, что и пустое состояние экрана.
 */
export default function Loading(): JSX.Element {
  return <PropertiesSearchLoading />;
}
