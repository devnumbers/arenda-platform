import type { Metadata } from 'next';
import { ParticipantsHubScreen } from '@/widgets/participants';

/**
 * Хаб «Совместный доступ» (карта #692, тикет #696): пункт навбара
 * «Участники» перестаёт быть заглушкой #559 — живой раздел с карточками
 * «Ваши участники» и «Объекты пользователей»; второй вход — строка в
 * профиле.
 */
export const metadata: Metadata = {
  title: 'Совместный доступ — Рентли',
};

export default function ParticipantsRoutePage() {
  return <ParticipantsHubScreen />;
}
