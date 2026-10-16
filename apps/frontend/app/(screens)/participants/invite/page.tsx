import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantsInviteScreen } from '@/widgets/participants';

/** Экран «Пригласите участника» (карта #692, тикет #699): мультиобъектный
 * флоу приглашения из хаба. */
export const metadata: Metadata = {
  title: 'Пригласить участника — Рентли',
};

export default function ParticipantsInviteRoutePage(): JSX.Element {
  return <ParticipantsInviteScreen />;
}
