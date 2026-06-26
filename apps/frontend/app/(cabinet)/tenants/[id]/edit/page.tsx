import type { Metadata } from 'next';
import { TenantEditForm } from '@/widgets/tenants';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Редактировать арендатора — Arenda Platform',
  description: 'Изменение информации об арендаторе',
};

interface TenantEditPageProps {
  params: Promise<{ id: string }>;
}

export default async function TenantEditPage({ params }: TenantEditPageProps) {
  const { id } = await params;

  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <TenantEditForm tenantId={id} />
      </div>
    </div>
  );
}
