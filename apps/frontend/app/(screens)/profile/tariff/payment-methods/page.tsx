import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PaymentMethodList } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Способы оплаты — Рентли',
  description: 'Управление способами оплаты',
};

export default async function PaymentMethodsPage({
  searchParams,
}: PageProps<'/profile/tariff/payment-methods'>) {
  const params = await searchParams;
  const addCardResult =
    typeof params.addCard === 'string' ? params.addCard : undefined;

  return (
    <>
      <SubScreenShell title="Способы оплаты" fallbackHref={ROUTES.profileTariff}>
        <PaymentMethodList addCardResult={addCardResult} />
      </SubScreenShell>
    </>
  );
}
