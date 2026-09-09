import type { Metadata } from 'next';
import { PageContent, TopNav, TopNavBackButton, TopNavTitle } from '@/shared/ui/design';
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
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.profileTariff} />}>
        <TopNavTitle title="Способы оплаты" />
      </TopNav>
      <PageContent className="px-6">
        <PaymentMethodList addCardResult={addCardResult} />
      </PageContent>
    </>
  );
}
