import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantScreen } from '@/widgets/participants';
import { participantQueryOptions } from '@/features/participants';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Страница участника (карта #692, тикет #698): блок участника, список
 * «Доступные объекты», кебаб с приглашением и отзывом.
 *
 * Без in-page Suspense (эксперимент #1605): ServerPrefetchBoundary ждёт
 * данные до рендера, маршрут без loading.tsx — HTML однофлушный, второй
 * флуш ($RC/$RB) не возникает, окно гонки гидратации react#37014 закрыто
 * по построению. */
export const metadata: Metadata = {
  title: 'Участник — Рентли',
};

export default async function ParticipantRoutePage({
  params,
}: PageProps<'/participants/[participantId]'>): Promise<JSX.Element> {
  const { participantId } = await params;

  return (
    <ServerPrefetchBoundary prefetch={(queryClient) => {
      void queryClient.prefetchQuery(participantQueryOptions({ participantId, transport: serverApiClient }));
    }}>
      <ParticipantScreen participantId={participantId} />
    </ServerPrefetchBoundary>
  );
}
