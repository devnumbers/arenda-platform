import type { Metadata } from 'next';
import { ParticipantsPlaceholderScreen } from '@/widgets/participants';

/** Каркас экрана «Ваши участники» (#697): список приедет тикетом. */
export const metadata: Metadata = {
  title: 'Ваши участники — Рентли',
};

export default function ParticipantsListRoutePage() {
  return <ParticipantsPlaceholderScreen title="Ваши участники" />;
}
