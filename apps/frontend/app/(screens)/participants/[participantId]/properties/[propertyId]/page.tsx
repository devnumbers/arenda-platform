import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantRightsScreen } from '@/widgets/participants';

/** Экран «Права участника» (карта #692, тикет #698): роль и отзыв
 * доступа на одном объекте. */
export const metadata: Metadata = {
  title: 'Права участника — Рентли',
};

type ParticipantRightsRoutePageProps = {
  params: Promise<{ participantId: string; propertyId: string }>;
};

export default async function ParticipantRightsRoutePage({
  params,
}: ParticipantRightsRoutePageProps): Promise<JSX.Element> {
  const { participantId, propertyId } = await params;

  return <ParticipantRightsScreen participantId={participantId} propertyId={propertyId} />;
}
