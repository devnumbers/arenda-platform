import { Suspense, type JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantLoading, ParticipantScreen } from '@/widgets/participants';
import { participantQueryOptions } from '@/features/participants';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/** Страница участника (карта #692, тикет #698): блок участника, список
 * «Доступные объекты», кебаб с приглашением и отзывом. */
export const metadata: Metadata = {
  title: 'Участник — Рентли',
};

export default async function ParticipantRoutePage({
  params,
}: PageProps<'/participants/[participantId]'>): Promise<JSX.Element> {
  const { participantId } = await params;

  return (
    <Suspense fallback={<ParticipantLoading />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => {
        void queryClient.prefetchQuery(participantQueryOptions({ participantId, transport: serverApiClient }));
      }}>
        <ParticipantScreen participantId={participantId} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
