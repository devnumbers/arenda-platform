import type { Metadata } from 'next';
import { ParticipantsPlaceholderScreen } from '@/widgets/participants';

/** Каркас экрана «Объекты пользователей» (#701): список приедет тикетом. */
export const metadata: Metadata = {
  title: 'Объекты пользователей — Рентли',
};

export default function ParticipantsPropertiesRoutePage() {
  return <ParticipantsPlaceholderScreen title="Объекты пользователей" />;
}
