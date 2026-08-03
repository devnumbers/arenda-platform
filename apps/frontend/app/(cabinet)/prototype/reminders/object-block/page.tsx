// PROTOTYPE — throwaway, issue #105

import type { Metadata } from 'next';
import type { JSX } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { parsePrototypeVariant } from '@/widgets/prototype-reminders/model/variant';
import { ObjectPageScaffold } from '@/widgets/prototype-reminders/ui/ObjectPageScaffold';
import { VariantAObjectBlock } from '@/widgets/prototype-reminders/ui/variant-a/VariantAObjectBlock';
import { VariantBObjectBlock } from '@/widgets/prototype-reminders/ui/variant-b/VariantBObjectBlock';
import { VariantCObjectBlock } from '@/widgets/prototype-reminders/ui/variant-c/VariantCObjectBlock';

export const metadata: Metadata = {
  title: 'Прототип: напоминания на объекте — Рентли',
  description: 'Прототип блока «Напоминания» на странице объекта (issue #105)',
};

export default async function PrototypeReminderObjectBlockPage({
  searchParams,
}: {
  readonly searchParams: Promise<{ variant?: string | string[] }>;
}): Promise<JSX.Element> {
  const params = await searchParams;
  const variant = parsePrototypeVariant(params.variant);

  return (
    <PageShell>
      <ObjectPageScaffold>
        {variant === 'A' && <VariantAObjectBlock variant={variant} />}
        {variant === 'B' && <VariantBObjectBlock variant={variant} />}
        {variant === 'C' && <VariantCObjectBlock variant={variant} />}
      </ObjectPageScaffold>
    </PageShell>
  );
}
