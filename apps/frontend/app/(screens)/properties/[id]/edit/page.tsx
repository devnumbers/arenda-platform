import type { Metadata } from 'next';
import { PropertyEditScreen } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Редактирование объекта — Рентли',
  description: 'Изменение информации об объекте недвижимости',
};

/** Правка объекта по новым макетам (карта #583, тикет #590): страница-
 * маршрут с полноэкранной формой — хедер с крестиком и галочкой
 * сохранения, поля и StickyBottomBar собирает клиентский экран. */
export default async function PropertyEditPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return <PropertyEditScreen propertyId={id} />;
}
