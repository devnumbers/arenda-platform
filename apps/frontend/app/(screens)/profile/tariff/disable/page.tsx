import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffDisableScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Отключение тарифа — Рентли',
  description: 'Флоу отключения тарифа',
};

/** Экран «Отключение тарифа» (карта #611, тикет #622): выбор сохраняемого
 * объекта (когда активных больше одного), подтверждение и успех. Точка
 * входа — «Отключить тариф» на «О тарифе» (#621); поверх контекста
 * «О тарифе». */
export default function TariffDisablePage() {
  return (
    <SubScreenShell title="Отключение тарифа" fallbackHref={ROUTES.profileTariffAbout}>
      <TariffDisableScreen />
    </SubScreenShell>
  );
}
