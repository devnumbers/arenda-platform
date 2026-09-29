import type { Metadata } from 'next';
import { RentalCreateWizardScreen } from '@/widgets/rentals';

/**
 * Визард создания аренды (#530): один маршрут, четыре шага — клиентское
 * состояние, черновик переживает перезагрузку и уход в ветку «Создать
 * контакт» (#509 с ?pick=rental): созданный контакт возвращается в
 * черновике, без параметров в адресе.
 */

export const metadata: Metadata = {
  title: 'Создание аренды — Рентли',
};

export default async function RentalNewRoutePage({ params }: PageProps<'/properties/[id]/rentals/new'>) {
  const { id } = await params;

  return <RentalCreateWizardScreen propertyId={id} />;
}
