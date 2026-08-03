// PROTOTYPE — throwaway, issue #106

import { Suspense, type JSX, type ReactNode } from 'react';
import { PrototypeCalendarScaffold } from '@/widgets/prototype-calendar/ui/PrototypeCalendarScaffold';
import styles from './layout.module.css';

export default function PrototypeCalendarLayout({
  children,
}: {
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <>
      <Suspense fallback={null}>
        <PrototypeCalendarScaffold />
      </Suspense>
      {/* отступ под фиксированную верхнюю полосу навигации прототипа */}
      <div className={styles.contentShift}>{children}</div>
    </>
  );
}
