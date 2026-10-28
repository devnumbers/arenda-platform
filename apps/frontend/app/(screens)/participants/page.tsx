import type { QueryClient } from '@tanstack/react-query';
import type { Metadata } from 'next';
import { ParticipantsListScreen, parseParticipantsListOrderParams } from '@/widgets/participants';
import { participantsListQueryOptions } from '@/features/participants';
import { subscriptionQueryOptions, type Subscription } from '@/features/subscription';
import { isPaidTariff } from '@/entities/user';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { subscriptionKeys } from '@/shared/api/query-keys';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Раздел «Участники» — единая страница списка (D1–D4 #1313): хаб-карточки
 * и «Объекты пользователей» снесены, /participants — сам список. Состояние
 * сортировки живёт в адресе (?order=, #785) — стартовое значение парсится
 * здесь, на сервере. Серверный префетч (§15, тикет #1422): гейт подписки
 * повторяется серверно (ADR 0064) — список раскладывается только на
 * платном тарифе; сбой подписки — платный фолбэк экрана (деградация
 * видима на списке), basic остаётся на апселле без запроса списка. */
export const metadata: Metadata = {
  title: 'Участники — Рентли',
};

async function prefetchParticipantsScreen(queryClient: QueryClient): Promise<void> {
  await queryClient.prefetchQuery(subscriptionQueryOptions(serverApiClient));
  if (queryClient.getQueryState(subscriptionKeys.subscription)?.status !== 'success') {
    return;
  }
  const subscription = queryClient.getQueryData<Subscription | null>(subscriptionKeys.subscription);
  if (subscription ? isPaidTariff(subscription.tariff.name) : true) {
    void queryClient.prefetchQuery(participantsListQueryOptions({ transport: serverApiClient }));
  }
}

export default async function ParticipantsRoutePage({
  searchParams,
}: PageProps<'/participants'>) {
  const resolved = await searchParams;
  const initialOrder = parseParticipantsListOrderParams(resolved.order);

  return (
<ServerPrefetchBoundary prefetch={prefetchParticipantsScreen}>
  <ParticipantsListScreen initialOrder={initialOrder} />
</ServerPrefetchBoundary>
  );
}
