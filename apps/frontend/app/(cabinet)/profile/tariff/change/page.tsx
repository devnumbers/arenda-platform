import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { TariffChangeForm } from '@/widgets/profile';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Сменить тариф — Arenda Platform',
  description: 'Выбор нового тарифа и периода оплаты',
};

export default function TariffChangePage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profileTariff}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Сменить тариф</h1>
        </header>
        <section className={styles.section}>
          <TariffChangeForm />
        </section>
      </div>
    </div>
  );
}
