import type { Metadata } from 'next';
import { PropertyEditForm } from '@/widgets/properties';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Редактировать объект — Arenda Platform',
  description: 'Изменение информации об объекте недвижимости',
};

export default async function PropertyEditPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <div className={styles.root}>
      <div className={styles.content}>
        <PropertyEditForm propertyId={id} />
      </div>
    </div>
  );
}
