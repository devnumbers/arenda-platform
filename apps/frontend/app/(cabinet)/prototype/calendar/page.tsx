// PROTOTYPE — throwaway, issue #106

import type { Metadata } from 'next';
import type { JSX } from 'react';
import { PageShell } from '@/shared/ui/page-shell';
import { parsePrototypeVariant } from '@/widgets/prototype-calendar/model/variant';
import { VariantAMonthGrid } from '@/widgets/prototype-calendar/ui/variant-a/VariantAMonthGrid';
import { VariantBWeekAgenda } from '@/widgets/prototype-calendar/ui/variant-b/VariantBWeekAgenda';
import { VariantCObjectFeed } from '@/widgets/prototype-calendar/ui/variant-c/VariantCObjectFeed';

export const metadata: Metadata = {
  title: 'Прототип: календарь напоминаний — Рентли',
  description: 'Прототип страницы календаря напоминаний в основной навигации кабинета (issue #106)',
};

export default async function PrototypeCalendarPage({
  searchParams,
}: {
  readonly searchParams: Promise<{ variant?: string | string[] }>;
}): Promise<JSX.Element> {
  const params = await searchParams;
  const variant = parsePrototypeVariant(params.variant);

  return (
    <PageShell>
      {variant === 'A' && <VariantAMonthGrid />}
      {variant === 'B' && <VariantBWeekAgenda />}
      {variant === 'C' && <VariantCObjectFeed />}
    </PageShell>
  );
}
