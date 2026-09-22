import type { Metadata } from 'next';
import { ParticipantsPropertiesScreen, parseParticipantsPropertyOrderParams } from '@/widgets/participants';

/** Экран «Объекты пользователей» (карта #692, тикет #701): чужие объекты
 * читающего, «Покинуть объект/все объекты». Заголовок шапки — «Доступные
 * объекты» по макету 2010-132145. Направление сортировки живёт в адресе
 * (?order=, #785) — стартовое значение парсится здесь, на сервере. */
export const metadata: Metadata = {
  title: 'Доступные объекты — Рентли',
};

export default async function ParticipantsPropertiesRoutePage({
  searchParams,
}: PageProps<'/participants/properties'>) {
  const resolved = await searchParams;
  const initialOrder = parseParticipantsPropertyOrderParams(resolved.order);

  return <ParticipantsPropertiesScreen initialOrder={initialOrder} />;
}
