import type { Metadata } from 'next';
import { ParticipantsListScreen, parseParticipantsListOrderParams } from '@/widgets/participants';

/** Раздел «Участники» — единая страница списка (D1–D4 #1313): хаб-карточки
 * и «Объекты пользователей» снесены, /participants — сам список. Состояние
 * сортировки живёт в адресе (?order=, #785) — стартовое значение парсится
 * здесь, на сервере. */
export const metadata: Metadata = {
  title: 'Участники — Рентли',
};

export default async function ParticipantsRoutePage({
  searchParams,
}: PageProps<'/participants'>) {
  const resolved = await searchParams;
  const initialOrder = parseParticipantsListOrderParams(resolved.order);

  return <ParticipantsListScreen initialOrder={initialOrder} />;
}
