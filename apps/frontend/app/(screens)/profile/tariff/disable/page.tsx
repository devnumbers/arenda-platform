import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';

export const metadata: Metadata = {
  title: 'Отключение тарифа — Рентли',
  description: 'Флоу отключения тарифа',
};

/** Нейтральный каркас флоу отключения тарифа (#622): кнопка «Отключить
 * тариф» экрана «О тарифе» (#621) уже ведёт сюда; содержимое флоу —
 * выбор объекта, подтверждение и успех — следующий тикет карты #611. */
export default function TariffDisablePage() {
  return (
    <SubScreenShell title="Отключение тарифа" fallbackHref={ROUTES.profileTariffAbout}>
      <div />
    </SubScreenShell>
  );
}
