import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeSuccess } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф изменён — Рентли',
  description: 'Подтверждение смены тарифа',
};

export default function TariffChangeSuccessPage() {
  return (
    <PageShell>
      <PageHeader title="Тариф изменён" backHref={ROUTES.profileTariff} />
      <TariffChangeSuccess />
    </PageShell>
  );
}
