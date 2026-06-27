import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { PersonalDataForm } from '@/widgets/profile/ui/PersonalDataForm';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Мои данные — Arenda Platform',
  description: 'Редактирование персональных данных',
};

export default function PersonalDataPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profile}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Мои данные</h1>
        </header>
        <section className={styles.section}>
          <PersonalDataForm />
        </section>
      </div>
    </div>
  );
}
