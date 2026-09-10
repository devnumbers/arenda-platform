import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeForm } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Сменить тариф — Рентли',
  description: 'Выбор нового тарифа и периода оплаты',
};

export default function TariffChangePage() {
  return (
    <>
      <SubScreenShell title="Сменить тариф" fallbackHref={ROUTES.profileTariff}>
        <TariffChangeForm />
      </SubScreenShell>
    </>
  );
}
