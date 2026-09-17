import type { JSX } from 'react';
import type { Metadata } from 'next';
import { ParticipantsPropertiesScreen } from '@/widgets/participants';

/** Экран «Объекты пользователей» (карта #692, тикет #701): чужие объекты
 * читающего, «Покинуть объект/все объекты». Заголовок шапки — «Доступные
 * объекты» по макету 2010-132145. */
export const metadata: Metadata = {
  title: 'Доступные объекты — Рентли',
};

export default function ParticipantsPropertiesRoutePage(): JSX.Element {
  return <ParticipantsPropertiesScreen />;
}
