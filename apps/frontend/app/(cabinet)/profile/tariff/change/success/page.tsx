import type { Metadata } from 'next';
import { TariffChangeSuccess } from '@/widgets/profile';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Тариф изменен — Arenda Platform',
  description: 'Подтверждение смены тарифа',
};

export default function TariffChangeSuccessPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <TariffChangeSuccess />
      </div>
    </div>
  );
}
