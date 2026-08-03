// PROTOTYPE — throwaway, issue #105

import type { Metadata } from 'next';
import type { JSX } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { parsePrototypeVariant } from '@/widgets/prototype-reminders/model/variant';
import { VariantACreate } from '@/widgets/prototype-reminders/ui/variant-a/VariantACreate';
import { VariantBCreate } from '@/widgets/prototype-reminders/ui/variant-b/VariantBCreate';
import { VariantCCreate } from '@/widgets/prototype-reminders/ui/variant-c/VariantCCreate';

export const metadata: Metadata = {
  title: 'Прототип: новое напоминание — Рентли',
  description: 'Прототип страницы создания свободного напоминания (issue #105)',
};

export default async function PrototypeReminderCreatePage({
  searchParams,
}: {
  readonly searchParams: Promise<{ variant?: string | string[]; object?: string | string[] }>;
}): Promise<JSX.Element> {
  const params = await searchParams;
  const variant = parsePrototypeVariant(params.variant);
  const preselectedObjectId = typeof params.object === 'string' ? params.object : undefined;

  return (
    <PageShell>
      {variant === 'A' && (
        <VariantACreate variant={variant} preselectedObjectId={preselectedObjectId} />
      )}
      {variant === 'B' && (
        <VariantBCreate variant={variant} preselectedObjectId={preselectedObjectId} />
      )}
      {variant === 'C' && (
        <VariantCCreate variant={variant} preselectedObjectId={preselectedObjectId} />
      )}
    </PageShell>
  );
}
