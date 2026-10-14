import { Suspense } from 'react';
import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { TariffChangeScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Выбрать тариф — Рентли',
  description: 'Выбор тарифа и периода оплаты подписки',
};

/** Страница выбора тарифа (#623). Сюда же ведёт редирект платного гейта
 * (карта #997): proxy.ts отправляет базовый тариф с платных разделов на
 * этот экран — без контекстной плашки, решение владельца 30.09. Экран
 * читает query глубокой ссылки лендинга (?tariff=…&period=…) через
 * useSearchParams — Suspense-граница обязательна для пререндера (Next). */
export default function TariffChangePage() {
  return (
    <SubScreenShell title="Выбрать тариф" fallbackHref={ROUTES.profileTariff}>
      <Suspense>
        <TariffChangeScreen />
      </Suspense>
    </SubScreenShell>
  );
}
