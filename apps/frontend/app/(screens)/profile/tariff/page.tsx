import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф — Рентли',
  description: 'Управление тарифом и подпиской',
};

export default function TariffPage() {
  return (
    <SubScreenShell title="Тариф" fallbackHref={ROUTES.profile}>
      <TariffScreen />
    </SubScreenShell>
  );
}
