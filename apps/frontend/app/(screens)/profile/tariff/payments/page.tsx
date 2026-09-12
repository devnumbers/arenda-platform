import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PaymentList } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Операции — Рентли',
  description: 'История оплат подписки',
};

/** Экран «Операции» (#624): название — копирайт-решение владельца;
 * домен — Subscription Payments, учётные «Операции» (доходы/расходы)
 * не смешиваются. */
export default function PaymentsPage() {
  return (
    <>
      <SubScreenShell title="Операции" fallbackHref={ROUTES.profileTariff}>
        <PaymentList />
      </SubScreenShell>
    </>
  );
}
