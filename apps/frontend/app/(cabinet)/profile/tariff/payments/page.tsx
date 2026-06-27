import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { PaymentList } from '@/widgets/profile';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'История операций — Arenda Platform',
  description: 'История операций по тарифу',
};

export default function PaymentsPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profileTariff}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>История операций</h1>
        </header>
        <PaymentList />
      </div>
    </div>
  );
}
