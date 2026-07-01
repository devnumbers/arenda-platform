import type { Metadata } from 'next';
import { TenantsPage } from '@/widgets/tenants';

export const metadata: Metadata = {
  title: 'Арендаторы — Рентли',
  description: 'Список арендаторов',
};

export default function TenantsListPage() {
  return <TenantsPage />;
}
