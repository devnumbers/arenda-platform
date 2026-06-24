'use client';

import type { JSX, ReactNode } from 'react';
import styles from './PaymentRow.module.css';

export type PaymentRowProps = {
  readonly icon: ReactNode;
  readonly title: string;
  readonly subtitle: string;
  readonly amount: string;
  readonly deadline: string;
};

export function PaymentRow({ icon, title, subtitle, amount, deadline }: PaymentRowProps): JSX.Element {
  return (
    <div className={styles.root}>
      <span className={styles.icon}>{icon}</span>
      <div className={styles.info}>
        <span className={styles.title}>{title}</span>
        <span className={styles.subtitle}>{subtitle}</span>
      </div>
      <div className={styles.meta}>
        <span className={styles.amount}>{amount}</span>
        <span className={styles.deadline}>{deadline}</span>
      </div>
    </div>
  );
}
