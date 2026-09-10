import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PaymentList } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'История платежей — Рентли',
  description: 'История платежей по тарифу',
};

export default function PaymentsPage() {
  return (
    <>
      <SubScreenShell title="История платежей" fallbackHref={ROUTES.profileTariff}>
        <PaymentList />
      </SubScreenShell>
    </>
  );
}
