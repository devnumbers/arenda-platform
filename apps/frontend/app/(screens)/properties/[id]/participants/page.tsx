import type { JSX } from 'react';
import type { Metadata } from 'next';
import { PropertyParticipantsScreen } from '@/widgets/participants';

/** Экран «Участники объекта» (карта #692, тикет #700): замена легаси-
 * модалки совместного доступа — список, поиск, роли, приглашение. */
export const metadata: Metadata = {
  title: 'Участники объекта — Рентли',
};

export default function PropertyParticipantsRoutePage(): JSX.Element {
  return <PropertyParticipantsScreen />;
}
