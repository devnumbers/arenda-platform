import type { Metadata } from 'next';
import { TenantCreateWizard } from '@/widgets/tenants';
import { PageShell } from '@/shared/ui/page-shell';

export const metadata: Metadata = {
  title: 'Добавить арендатора — Arenda Platform',
  description: 'Добавление нового арендатора',
};

export default function TenantsNewPage() {
  return (
    <PageShell>
      <TenantCreateWizard />
    </PageShell>
  );
}
