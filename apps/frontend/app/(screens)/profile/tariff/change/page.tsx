import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Выбрать тариф — Рентли',
  description: 'Выбор тарифа и периода оплаты подписки',
};

export default function TariffChangePage() {
  return (
    <>
      <SubScreenShell title="Выбрать тариф" fallbackHref={ROUTES.profileTariff}>
        <TariffChangeScreen />
      </SubScreenShell>
    </>
  );
}
