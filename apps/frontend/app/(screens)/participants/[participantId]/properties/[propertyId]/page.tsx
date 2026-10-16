import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantRightsScreen } from '@/widgets/participants';

/** Экран «Права участника» (карта #692, тикет #698): роль и отзыв
 * доступа на одном объекте. */
export const metadata: Metadata = {
  title: 'Права участника — Рентли',
};

export default async function ParticipantRightsRoutePage({
  params,
}: PageProps<'/participants/[participantId]/properties/[propertyId]'>): Promise<JSX.Element> {
  const { participantId, propertyId } = await params;

  return <ParticipantRightsScreen participantId={participantId} propertyId={propertyId} />;
}
