import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { PaymentMethodList } from '@/widgets/profile';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Способы оплаты — Arenda Platform',
  description: 'Управление способами оплаты',
};

export default function PaymentMethodsPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profileTariff}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Способы оплаты</h1>
        </header>
        <PaymentMethodList />
      </div>
    </div>
  );
}
