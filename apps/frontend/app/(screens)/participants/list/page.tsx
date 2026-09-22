import type { Metadata } from 'next';
import { ParticipantsListScreen, parseParticipantsListOrderParams } from '@/widgets/participants';

/** Экран «Ваши участники» (карта #692, тикет #697): список агрегатов,
 * поиск, «Отозвать всех». Направление сортировки живёт в адресе (?order=,
 * #785) — стартовое значение парсится здесь, на сервере. */
export const metadata: Metadata = {
  title: 'Ваши участники — Рентли',
};

export default async function ParticipantsListRoutePage({
  searchParams,
}: PageProps<'/participants/list'>) {
  const resolved = await searchParams;
  const initialOrder = parseParticipantsListOrderParams(resolved.order);

  return <ParticipantsListScreen initialOrder={initialOrder} />;
}
