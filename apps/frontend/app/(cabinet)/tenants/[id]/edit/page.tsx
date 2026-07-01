import type { Metadata } from 'next';
import { TenantEditForm } from '@/widgets/tenants';
import { PageShell } from '@/shared/ui/page-shell';

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
    <PageShell>
      <TenantEditForm tenantId={id} />
    </PageShell>
  );
}
