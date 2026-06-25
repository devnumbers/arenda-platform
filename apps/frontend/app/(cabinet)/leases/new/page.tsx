import type { Metadata } from 'next';
import { LeaseCreateWizard } from '@/widgets/leases';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Создать аренду — Arenda Platform',
};

type LeasesNewPageProps = {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default async function LeasesNewPage({ searchParams }: LeasesNewPageProps) {
  const params = await searchParams;
  const rawPropertyId = params.propertyId;
  const propertyId = Array.isArray(rawPropertyId) ? rawPropertyId[0] : rawPropertyId;

  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <LeaseCreateWizard propertyId={propertyId} />
      </div>
    </div>
  );
}
