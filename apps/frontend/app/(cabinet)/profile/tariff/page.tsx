import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { TariffOverview } from '@/widgets/profile';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Тариф — Arenda Platform',
  description: 'Управление тарифом и подпиской',
};

export default function TariffPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profile}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Тариф</h1>
        </header>
        <TariffOverview />
      </div>
    </div>
  );
}
