import type { JSX } from 'react';
import type { Metadata } from 'next';
import { PaymentsGlobalScreen } from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import {
  globalPaymentObjectsQueryOptions,
  globalPaymentsFeedQueryOptions,
  paymentsDueQueryOptions,
} from '@/features/payments';
import { propertiesListQueryOptions } from '@/features/properties';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

export const metadata: Metadata = { title: 'Платежи — Рентли' };

/**
 * Хаб «Платежи» — пионер серверного префетча #887: холодный вход рисует
 * первый кадр с данными. Граница живёт в page.tsx без Suspense-обёртки
 * (аменда #1605 — однофлушный HTML); hover-префетч динамики рендерит RSC
 * только до границы и серверные запросы на наводке впустую не гоняет
 * (research #863).
 *
 * Полный кадр (тикет #1602, канон §15 «первый кадр читает → префетчит»):
 * экран читает четыре ключа — лента, стопки объектов, «На оплату»
 * (#1346/#1347) и справочник объектов (кнопка «+», #703). Раньше первые
 * два слоя префетча (#887 здесь и прогрев #626) покрывали только ленту со
 * стопками — «На оплату» всегда догружалась клиентом, и на входе без
 * данных в кэше (холодный вход, возврат после gcTime-выселения) виджет
 * играл «скелетон → контент» — видимое «всё перезагрузилось» (#1593).
 */
export default function PaymentsRoutePage(): JSX.Element {
  return (
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    void queryClient.prefetchQuery(globalPaymentsFeedQueryOptions(serverApiClient));
    void queryClient.prefetchQuery(globalPaymentObjectsQueryOptions({ transport: serverApiClient }));
    void queryClient.prefetchQuery(paymentsDueQueryOptions(serverApiClient));
    void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
  }}
>
  <PaymentsGlobalScreen />
</ServerPrefetchBoundary>
  );
}
