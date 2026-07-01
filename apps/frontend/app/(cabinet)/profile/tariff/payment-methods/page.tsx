import type { Metadata } from 'next';
import { PageHeader } from '@/shared/ui/page-header';
import { PageShell } from '@/shared/ui/page-shell';
import { ROUTES } from '@/shared/config/routes';
import { PaymentMethodList } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Способы оплаты — Рентли',
  description: 'Управление способами оплаты',
};

type PaymentMethodsPageProps = {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PaymentMethodsPage({
  searchParams,
}: PaymentMethodsPageProps) {
  const params = await searchParams;
  const addCardResult =
    typeof params.addCard === 'string' ? params.addCard : undefined;

  return (
    <PageShell>
      <PageHeader title="Способы оплаты" backHref={ROUTES.profileTariff} />
      <PaymentMethodList addCardResult={addCardResult} />
    </PageShell>
  );
}
