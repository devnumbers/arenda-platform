import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeForm } from '@/widgets/profile';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Сменить тариф — Arenda Platform',
  description: 'Выбор нового тарифа и периода оплаты',
};

export default function TariffChangePage() {
  return (
    <PageShell>
      <PageHeader title="Сменить тариф" backHref={ROUTES.profileTariff} />
      <section className={styles.section}>
        <TariffChangeForm />
      </section>
    </PageShell>
  );
}
