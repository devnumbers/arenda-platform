import type { JSX, ReactNode } from 'react';
import styles from './PropertyDetailSection.module.css';

export function PropertyDetailSection({
  children,
}: {
  readonly children: ReactNode;
}): JSX.Element {
  return <section className={styles.section}>{children}</section>;
}
