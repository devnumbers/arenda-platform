// PROTOTYPE — throwaway, issue #105

import type { Metadata } from 'next';
import type { JSX } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { parsePrototypeVariant } from '@/widgets/prototype-reminders/model/variant';
import { VariantAItem } from '@/widgets/prototype-reminders/ui/variant-a/VariantAItem';
import { VariantBItem } from '@/widgets/prototype-reminders/ui/variant-b/VariantBItem';
import { VariantCItem } from '@/widgets/prototype-reminders/ui/variant-c/VariantCItem';

export const metadata: Metadata = {
  title: 'Прототип: напоминание — Рентли',
  description: 'Прототип страницы просмотра, редактирования и удаления напоминания (issue #105)',
};

export default async function PrototypeReminderItemPage({
  searchParams,
}: {
  readonly searchParams: Promise<{ variant?: string | string[] }>;
}): Promise<JSX.Element> {
  const params = await searchParams;
  const variant = parsePrototypeVariant(params.variant);

  return (
    <PageShell>
      {variant === 'A' && <VariantAItem variant={variant} />}
      {variant === 'B' && <VariantBItem variant={variant} />}
      {variant === 'C' && <VariantCItem variant={variant} />}
    </PageShell>
  );
}
