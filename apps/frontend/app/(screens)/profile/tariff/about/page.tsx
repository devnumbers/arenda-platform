import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffAboutScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'О тарифе — Рентли',
  description: 'Подробности тарифа и подписки',
};

/** Экран «О тарифе» (карта #611, тикет #621): карточка тарифа,
 * «Возможности», отключение с гардом pending, возобновление и футер
 * «Выбрать другой тариф». Точка входа — hero-карточка главного
 * «Тарифа». */
export default function TariffAboutPage() {
  return (
    <SubScreenShell title="О тарифе" fallbackHref={ROUTES.profileTariff}>
      <TariffAboutScreen />
    </SubScreenShell>
  );
}
