import type { Metadata } from 'next';
import { TenantDetailPage } from '@/widgets/tenant-detail';
import { PageShell } from '@/shared/ui/page-shell';

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
    <PageShell>
      <TenantDetailPage id={id} />
    </PageShell>
  );
}
