import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { PaymentList } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'История операций — Рентли',
  description: 'История операций по тарифу',
};

export default function PaymentsPage() {
  return (
    <PageShell>
      <PageHeader title="История операций" backHref={ROUTES.profileTariff} />
      <PaymentList />
    </PageShell>
  );
}
