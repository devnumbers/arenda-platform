import type { Metadata } from 'next';
import { RentalContactPickerScreen } from '@/widgets/rentals';

/** Экран выбора арендатора (#807, макет 1855:64129) — отдельный маршрут
 * шага «Контакт арендатора» визарда создания аренды (#530). Оболочка новых
 * экранов (ScreenLayout, колонка 560) — из layout группы (screens). */
export const metadata: Metadata = {
  title: 'Выбор арендатора — Рентли',
};

export default async function RentalContactPickerRoutePage(
  { params }: PageProps<'/properties/[id]/rentals/new/contact'>,
) {
  const { id } = await params;

  return <RentalContactPickerScreen propertyId={id} />;
}
