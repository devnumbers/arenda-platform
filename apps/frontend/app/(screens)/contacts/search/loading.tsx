import type { JSX } from 'react';
import { ContactSearchLoading } from '@/widgets/contacts';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же кадр, что и fallback Suspense-границы страницы.
 */
export default function Loading(): JSX.Element {
  return <ContactSearchLoading />;
}
