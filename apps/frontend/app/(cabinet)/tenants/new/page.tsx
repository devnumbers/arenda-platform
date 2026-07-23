import type { Metadata } from 'next';
import { TenantCreateWizard } from '@/widgets/tenants';
import { PageShell } from '@/shared/ui/page-shell';
import { sanitizeReturnTo } from '@/shared/lib/navigation';

export const metadata: Metadata = {
  title: 'Добавить арендатора — Рентли',
  description: 'Добавление нового арендатора',
};

export default async function TenantsNewPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { returnTo } = await searchParams;
  const sanitizedReturnTo = sanitizeReturnTo(typeof returnTo === 'string' ? returnTo : undefined);

  return (
    <PageShell>
      <TenantCreateWizard returnTo={sanitizedReturnTo} />
    </PageShell>
  );
}
