import type { Metadata } from 'next';
import { PropertyCreateWizard } from '@/widgets/properties';
import { PageShell } from '@/shared/ui/page-shell';

export const metadata: Metadata = {
  title: 'Создать объект — Arenda Platform',
  description: 'Добавление нового объекта недвижимости',
};

export default function PropertiesNewPage() {
  return (
    <PageShell>
      <PropertyCreateWizard />
    </PageShell>
  );
}
