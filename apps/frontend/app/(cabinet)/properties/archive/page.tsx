import type { Metadata } from 'next';
import { Suspense } from 'react';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { PropertiesLoading, PropertiesPage } from '@/widgets/properties';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Архивные объекты — Arenda Platform',
  description: 'Архивные объекты недвижимости',
};

export default function PropertiesArchivePage() {
  return (
    <div>
      <header className={styles.header}>
        <IconLink
          href={ROUTES.properties}
          aria-label="Назад"
          icon={<ArrowLeft />}
        />
        <h1 className={styles.title}>Архивные объекты</h1>
      </header>
      <Suspense fallback={<PropertiesLoading />}>
        <PropertiesPage mode="archived" hideTitle />
      </Suspense>
    </div>
  );
}
