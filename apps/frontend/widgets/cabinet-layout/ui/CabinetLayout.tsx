'use client';

import { useRef, type JSX, type ReactNode } from 'react';
import { RemindersOnboardingModal } from './RemindersOnboardingModal';
import { ServiceWorkerRegister } from '@/shared/lib/pwa/ServiceWorkerRegister';
import { ServiceWorkerUpdater } from '@/shared/lib/pwa/ServiceWorkerUpdater';
import { PullToRefresh } from '@/shared/ui/pull-to-refresh';
import { PushPermissionGate } from './PushPermissionGate';
import { BottomNav } from './BottomNav';
import { Sidebar } from './Sidebar';
import styles from './CabinetLayout.module.css';

export type CabinetLayoutProps = {
  readonly children: ReactNode;
};

export function CabinetLayout({ children }: CabinetLayoutProps): JSX.Element {
  // PullToRefresh drives `transform` on the content node during the gesture,
  // so the layout shares its ref with the component.
  const contentRef = useRef<HTMLDivElement>(null);

  return (
    <div className={styles.root}>
      <Sidebar />
      <main className={styles.main}>
        <div className={styles.content} ref={contentRef}>{children}</div>
      </main>
      <RemindersOnboardingModal />
      <BottomNav />
      <ServiceWorkerRegister />
      <ServiceWorkerUpdater />
      <PullToRefresh contentRef={contentRef} />
      <PushPermissionGate />
    </div>
  );
}
