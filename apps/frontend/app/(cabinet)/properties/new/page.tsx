import type { Metadata } from 'next';
import { PropertyCreateWizard } from '@/widgets/properties';
import styles from './page.module.css';

export const metadata: Metadata = {
  title: 'Создать объект — Arenda Platform',
  description: 'Добавление нового объекта недвижимости',
};

export default function PropertiesNewPage() {
  return (
    <div className={styles.root}>
      <PropertyCreateWizard />
    </div>
  );
}
