import type { Metadata } from 'next';
import { PageShell } from '@/shared/ui/page-shell';
import { PropertyEditForm } from '@/widgets/properties';

export const metadata: Metadata = {
  title: 'Редактировать объект — Рентли',
  description: 'Изменение информации об объекте недвижимости',
};

export default async function PropertyEditPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <PageShell>
      <PropertyEditForm propertyId={id} />
    </PageShell>
  );
}
