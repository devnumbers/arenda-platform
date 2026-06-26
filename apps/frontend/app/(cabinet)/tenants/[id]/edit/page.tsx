import { Metadata } from 'next';

import { TenantEditForm } from '@/widgets/tenants/ui';

import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Редактирование арендатора',
};

interface TenantEditPageProps {
  params: Promise<{ id: string }>;
}

export default async function TenantEditPage({ params }: TenantEditPageProps) {
  const { id } = await params;

  return (
    <div className={styles.page}>
      <div className={styles.container}>
        <h1 className={styles.title}>Редактирование арендатора</h1>
        <TenantEditForm tenantId={id} />
      </div>
    </div>
  );
}
