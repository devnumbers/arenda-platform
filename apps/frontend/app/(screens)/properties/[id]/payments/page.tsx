import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { PaymentsOfPropertyScreen } from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { propertyKeys } from '@/shared/api/query-keys';
import {
  paymentListQueryOptions,
  paymentOperationsOverdueQueryOptions,
} from '@/features/payments';
import { propertyDetailQueryOptions } from '@/features/properties';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер (аменда #1605 — однофлушный HTML без in-page Suspense,
// react/react#37014).
export const dynamic = 'force-dynamic';

/** Экран «Платежи объекта» (#463): секции «Просроченные», «Платежи»,
 * «Автоплатежи», поиск, шит выбора «Платёж / Автоплатёж». Оболочка новых
 * экранов (глобальный top-header на десктопе, колонка 560) — из layout
 * группы (screens). */
export const metadata: Metadata = {
  title: 'Платежи объекта — Рентли',
};

/**
 * Серверная раскладка первого кадра (тикет #1602, канон §15 «первый кадр
 * читает → префетчит»): экран читает деталь объекта, список платежей и
 * порцию просрочки. Гейт #769 повторяется серверно, как на странице
 * объекта: деталь решает читаемость, зависимые секции раскладываются
 * только после успеха — на нечитаемом объекте секции не простреливают
 * 404-волнами. Без префетча на входе без данных в кэше (холодный вход,
 * возврат после gcTime-выселения) виджет играл «скелетон → контент» —
 * видимое «всё перезагрузилось» (#1593).
 */
async function prefetchPaymentsOfPropertyScreen(queryClient: QueryClient, id: string): Promise<void> {
  await queryClient.prefetchQuery(propertyDetailQueryOptions({ id, transport: serverApiClient }));
  if (queryClient.getQueryState(propertyKeys.detail(id))?.status !== 'success') {
    return;
  }

  void queryClient.prefetchQuery(paymentListQueryOptions({ propertyId: id, transport: serverApiClient }));
  void queryClient.prefetchQuery(paymentOperationsOverdueQueryOptions({ propertyId: id, transport: serverApiClient }));
}

export default async function PaymentsOfPropertyRoutePage({
  params,
}: PageProps<'/properties/[id]/payments'>) {
  const { id } = await params;

  return (
    <ServerPrefetchBoundary
      prefetch={(queryClient) => prefetchPaymentsOfPropertyScreen(queryClient, id)}
    >
      <PaymentsOfPropertyScreen propertyId={id} />
    </ServerPrefetchBoundary>
  );
}
