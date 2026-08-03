// PROTOTYPE — throwaway, issue #105

import { Suspense, type JSX, type ReactNode } from 'react';
import { PrototypeScaffold } from '@/widgets/prototype-reminders/ui/PrototypeScaffold';
import styles from './layout.module.css';

export default function PrototypeRemindersLayout({
  children,
}: {
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <>
      <Suspense fallback={null}>
        <PrototypeScaffold />
      </Suspense>
      {/* отступ под фиксированную верхнюю полосу навигации прототипа */}
      <div className={styles.contentShift}>{children}</div>
    </>
  );
}
