import { Suspense } from 'react';
import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeSuccess } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф изменён — Рентли',
  description: 'Подтверждение смены тарифа',
};

export default function TariffChangeSuccessPage() {
  return (
    <>
      <SubScreenShell title="Тариф изменён" fallbackHref={ROUTES.profileTariff}>
        <Suspense fallback={null}>
          <TariffChangeSuccess />
        </Suspense>
      </SubScreenShell>
    </>
  );
}
