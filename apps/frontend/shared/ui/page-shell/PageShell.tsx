import type { JSX, ReactNode } from 'react';
import clsx from 'clsx';
import styles from './PageShell.module.css';

export interface PageShellProps {
  children: ReactNode;
  className?: string;
}

export function PageShell({ children, className }: PageShellProps): JSX.Element {
  return (
    <div className={clsx(styles.root, className)}>
      <div className={styles.content}>{children}</div>
    </div>
  );
}
