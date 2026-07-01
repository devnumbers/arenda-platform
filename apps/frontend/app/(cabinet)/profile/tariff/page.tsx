import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { TariffOverview } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф — Рентли',
  description: 'Управление тарифом и подпиской',
};

export default function TariffPage() {
  return (
    <PageShell>
      <PageHeader title="Тариф" backHref={ROUTES.profile} />
      <TariffOverview />
    </PageShell>
  );
}
