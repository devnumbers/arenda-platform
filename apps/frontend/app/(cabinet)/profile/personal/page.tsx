import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { PersonalDataForm } from '@/widgets/profile/ui/PersonalDataForm';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Мои данные — Рентли',
  description: 'Редактирование персональных данных',
};

export default function PersonalDataPage() {
  return (
    <PageShell>
      <PageHeader title="Мои данные" backHref={ROUTES.profile} />
      <section className={styles.section}>
        <PersonalDataForm />
      </section>
    </PageShell>
  );
}
