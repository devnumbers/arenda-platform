import type { Metadata } from 'next';
import { ParticipantsStubScreen } from '@/widgets/participants';

/**
 * Глобальные «Участники» — страница-заглушка единого хрома (карта #556,
 * тикет #559): пункт «Участники» главной навигации ведёт на живой маршрут;
 * фича совместного доступа (ADR 0028) — вне карты.
 */
export const metadata: Metadata = {
  title: 'Участники — Рентли',
};

export default function ParticipantsRoutePage() {
  return <ParticipantsStubScreen />;
}
