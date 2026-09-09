import type { Metadata } from 'next';
import { SubScreenShell } from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { PropertyEditForm } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Редактировать объект — Рентли',
  description: 'Изменение информации об объекте недвижимости',
};

/** Правка объекта на едином хроме (карта #556, снос кабинета #568):
 * каркас подэкрана собирает страница, форма — только содержимое. */
export default async function PropertyEditPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <SubScreenShell title="Информация об объекте" fallbackHref={ROUTES.property(id)}>
      <PropertyEditForm propertyId={id} />
    </SubScreenShell>
  );
}
