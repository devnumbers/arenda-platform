import type { JSX } from 'react';
import type { Metadata } from 'next';
import { PropertyParticipantsInviteScreen } from '@/widgets/participants';

/** Приглашение от объекта (карта #692, тикет #700): почта и роль —
 * объект фиксирован, выбора объектов нет (макет 1978-103001). */
export const metadata: Metadata = {
  title: 'Пригласите участника — Рентли',
};

export default function PropertyParticipantsInviteRoutePage(): JSX.Element {
  return <PropertyParticipantsInviteScreen />;
}
