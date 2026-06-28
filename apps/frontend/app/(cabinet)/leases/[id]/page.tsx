import type { Metadata } from 'next';
import { LeaseDetailPage } from '@/widgets/leases';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Аренда — Arenda Platform',
  description: 'Просмотр и редактирование аренды',
};

export default async function LeasePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <LeaseDetailPage id={id} />
      </div>
    </div>
  );
}
