import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantScreen } from '@/widgets/participants';

/** Страница участника (карта #692, тикет #698): блок участника, список
 * «Доступные объекты», кебаб с приглашением и отзывом. */
export const metadata: Metadata = {
  title: 'Участник — Рентли',
};

type ParticipantRoutePageProps = {
  params: Promise<{ participantId: string }>;
};

export default async function ParticipantRoutePage({
  params,
}: ParticipantRoutePageProps): Promise<JSX.Element> {
  const { participantId } = await params;

  return <ParticipantScreen participantId={participantId} />;
}
