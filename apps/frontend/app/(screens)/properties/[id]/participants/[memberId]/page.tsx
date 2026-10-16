import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantRightsMemberScreen } from '@/widgets/participants';

/** Экран «Права участника» в member-режиме (тикет #1362): read-only
 * страница прав для приглашённых — чтение одной строки участников
 * (#1357) по адресующему id из URL (membership id, invitation id у
 * pending или user_id у ряда владельца); deep-link и refresh участника
 * работают по построению. */
export const metadata: Metadata = {
  title: 'Права участника — Рентли',
};

export default async function PropertyParticipantRightsRoutePage({
  params,
}: PageProps<'/properties/[id]/participants/[memberId]'>): Promise<JSX.Element> {
  const { id, memberId } = await params;

  return <ParticipantRightsMemberScreen propertyId={id} memberId={memberId} />;
}
