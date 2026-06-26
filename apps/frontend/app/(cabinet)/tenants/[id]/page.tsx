import type { Metadata } from 'next';
import { TenantDetailPage } from '@/widgets/tenant-detail';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Арендатор — Arenda Platform',
  description: 'Просмотр данных арендатора',
};

export default async function TenantPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <TenantDetailPage id={id} />
      </div>
    </div>
  );
}
