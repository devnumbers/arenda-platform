import type { Metadata } from 'next';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { PhoneChangeForm } from '@/widgets/profile';
import { ROUTES } from '@/shared/config/routes';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Изменение телефона — Arenda Platform',
  description: 'Изменение номера телефона пользователя',
};

export default function ChangePhonePage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <header className={styles.header}>
          <IconLink
            href={ROUTES.profileAccount}
            aria-label="Назад"
            icon={<ArrowLeft />}
          />
          <h1 className={styles.title}>Изменение телефона</h1>
        </header>
        <section className={styles.section}>
          <PhoneChangeForm />
        </section>
      </div>
    </div>
  );
}
