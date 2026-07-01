import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { PhoneChangeForm } from '@/widgets/profile';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Изменение телефона — Arenda Platform',
  description: 'Изменение номера телефона пользователя',
};

export default function ChangePhonePage() {
  return (
    <PageShell>
      <PageHeader title="Изменение телефона" backHref={ROUTES.profileAccount} />
      <section className={styles.section}>
        <PhoneChangeForm />
      </section>
    </PageShell>
  );
}
