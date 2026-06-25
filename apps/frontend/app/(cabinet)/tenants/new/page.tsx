import type { Metadata } from 'next';
import { TenantCreateWizard } from '@/widgets/tenants';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Добавить арендатора — Arenda Platform',
  description: 'Добавление нового арендатора',
};

export default function TenantsNewPage() {
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <TenantCreateWizard />
      </div>
    </div>
  );
}
