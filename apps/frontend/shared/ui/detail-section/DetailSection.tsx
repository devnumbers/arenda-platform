import type { JSX, ReactNode } from 'react';
import styles from './DetailSection.module.css';

export function DetailSection({
  children,
}: {
  readonly children: ReactNode;
}): JSX.Element {
  return <section className={styles.section}>{children}</section>;
}
