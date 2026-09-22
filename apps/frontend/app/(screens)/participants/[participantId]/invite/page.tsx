import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantInviteScreen } from '@/widgets/participants';

/** Экран «Пригласить в объект» (карта #692, тикет #698): мультичек
 * объектов для существующего участника (POST /participants/{id}/properties,
 * #694). Не путать с /participants/invite — приглашением нового человека
 * (#699). */
export const metadata: Metadata = {
  title: 'Пригласить в объект — Рентли',
};

type ParticipantInviteRoutePageProps = {
  params: Promise<{ participantId: string }>;
};

export default async function ParticipantInviteRoutePage({
  params,
}: ParticipantInviteRoutePageProps): Promise<JSX.Element> {
  const { participantId } = await params;

  return <ParticipantInviteScreen participantId={participantId} />;
}
