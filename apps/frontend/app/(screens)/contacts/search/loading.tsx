import type { JSX } from 'react';
import { ContactSearchLoading } from '@/widgets/contacts';

/**
 * Route-loading сегмента (#609): архетип живёт в зоне виджета — тот же
 * кадр, что у экрана в idle-фазе поиска (hint); Suspense-границы в
 * page.tsx нет, экран держит pending-гейт сам.
 */
export default function Loading(): JSX.Element {
  return <ContactSearchLoading />;
}
