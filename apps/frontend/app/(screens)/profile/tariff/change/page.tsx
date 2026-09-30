import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { paidGateNoticeVisible, TariffChangeScreen } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Выбрать тариф — Рентли',
  description: 'Выбор тарифа и периода оплаты подписки',
};

/** Страница выбора тарифа (#623) с плашкой-контекстом платного гейта
 * (карта #997): proxy.ts редиректит базовый тариф с платных разделов сюда
 * с query `gate`, плашка объясняет, зачем пользователь оказался на экране.
 * Плашка серверная — видна в первом кадре, до загрузки данных экрана. */
export default async function TariffChangePage({
  searchParams,
}: PageProps<'/profile/tariff/change'>) {
  const resolved = await searchParams;
  const showPaidGateNotice = paidGateNoticeVisible(resolved.gate);

  return (
    <>
      <SubScreenShell title="Выбрать тариф" fallbackHref={ROUTES.profileTariff}>
        {showPaidGateNotice && (
          <div
            role="note"
            className="mb-4 rounded-3xl bg-surface-muted px-6 py-4 text-sm leading-4 text-content-secondary"
          >
            Эта функция доступна на платных тарифах
          </div>
        )}
        <TariffChangeScreen />
      </SubScreenShell>
    </>
  );
}
