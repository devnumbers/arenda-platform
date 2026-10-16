import type { Metadata } from 'next';
import { ParticipantsSearchScreen } from '@/widgets/participants';

/** Поиск по разделу «Участники» (#1313): отдельная страница с поисковой
 * шапкой — канон книги контактов (#1238); пилюля хаба и лупа компакт-бара
 * ведут сюда. Вложенный маршрут раздела — за тарифным гейтом (ADR 0064). */
export const metadata: Metadata = {
  title: 'Поиск участников — Рентли',
};

export default function ParticipantsSearchRoutePage() {
  return <ParticipantsSearchScreen />;
}
