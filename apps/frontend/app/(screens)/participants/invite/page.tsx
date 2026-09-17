import type { Metadata } from 'next';
import { ParticipantsPlaceholderScreen } from '@/widgets/participants';

/** Каркас экрана приглашения (#699): мультиобъектный флоу приедет тикетом. */
export const metadata: Metadata = {
  title: 'Пригласить участника — Рентли',
};

export default function ParticipantsInviteRoutePage() {
  return <ParticipantsPlaceholderScreen title="Пригласить участника" />;
}
