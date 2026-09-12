import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';

export const metadata: Metadata = {
  title: 'О тарифе — Рентли',
  description: 'Подробности тарифа и подписки',
};

/** Нейтральный каркас под главный экран «Тариф» (#620): точка входа из
 * hero-карточки; содержимое экрана «О тарифе» — тикет #621, следующий
 * в карте #611. */
export default function TariffAboutPage() {
  return (
    <SubScreenShell title="О тарифе" fallbackHref={ROUTES.profileTariff}>
      <div />
    </SubScreenShell>
  );
}
