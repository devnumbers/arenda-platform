import { Suspense } from 'react';
import type { Metadata } from 'next';
import { ParticipantsHubLoading, ParticipantsHubScreen } from '@/widgets/participants';
import { participantsSummaryQueryOptions } from '@/features/participants';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/**
 * Хаб «Совместный доступ» (карта #692, тикет #696): пункт навбара
 * «Участники» перестаёт быть заглушкой #559 — живой раздел с карточками
 * «Ваши участники» и «Объекты пользователей»; второй вход — строка в
 * профиле.
 */
export const metadata: Metadata = {
  title: 'Совместный доступ — Рентли',
};

export default function ParticipantsRoutePage() {
  return (
    <Suspense fallback={<ParticipantsHubLoading />}>
      <ServerPrefetchBoundary
        prefetch={(queryClient) => {
          void queryClient.prefetchQuery(participantsSummaryQueryOptions(serverApiClient));
        }}
      >
        <ParticipantsHubScreen />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
