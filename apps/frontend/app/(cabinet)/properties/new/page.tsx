import type { Metadata } from 'next';
import { PropertyCreateWizard } from '@/widgets/properties';
import { PageShell } from '@/shared/ui/page-shell';
import { sanitizeReturnTo } from '@/shared/lib/navigation';

export const metadata: Metadata = {
  title: 'Создать объект — Рентли',
  description: 'Добавление нового объекта недвижимости',
};

export default async function PropertiesNewPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { returnTo } = await searchParams;
  const sanitizedReturnTo = sanitizeReturnTo(typeof returnTo === 'string' ? returnTo : undefined);

  return (
    <PageShell>
      <PropertyCreateWizard returnTo={sanitizedReturnTo} />
    </PageShell>
  );
}
