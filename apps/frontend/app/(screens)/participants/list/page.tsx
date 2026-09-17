import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantsListScreen } from '@/widgets/participants';

/** Экран «Ваши участники» (карта #692, тикет #697): список агрегатов,
 * поиск, «Отозвать всех». */
export const metadata: Metadata = {
  title: 'Ваши участники — Рентли',
};

export default function ParticipantsListRoutePage(): JSX.Element {
  return <ParticipantsListScreen />;
}
