'use client';

import type { JSX, ReactNode } from 'react';
import { BottomNav } from './BottomNav';
import { Sidebar } from './Sidebar';
import styles from './CabinetLayout.module.css';

export type CabinetLayoutProps = {
  readonly children: ReactNode;
};

export function CabinetLayout({ children }: CabinetLayoutProps): JSX.Element {
  return (
    <div className={styles.root}>
      <Sidebar />
      <main className={styles.main}>
        <div className={styles.content}>{children}</div>
      </main>
      <BottomNav />
    </div>
  );
}

