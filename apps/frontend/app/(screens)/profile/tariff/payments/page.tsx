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
 * не смешиваются. Боковой инсет 24 несёт контент (строки Row Button,
 * заголовки дат px-6) — как у учётной «Истории платежей», поэтому
 * px-6 шела гасится. */
export default function PaymentsPage() {
  return (
    <>
      <SubScreenShell
        title="Операции"
        fallbackHref={ROUTES.profileTariff}
        contentClassName="px-0"
      >
        <PaymentList />
      </SubScreenShell>
    </>
  );
}
