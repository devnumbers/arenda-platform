import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { parsePaymentDraftTypeParam } from '@/features/payments';
import { PaymentCreateWizardScreen } from '@/widgets/payments';

/**
 * Визард создания платежа (#464): один маршрут, шаги — клиентское состояние,
 * тип «Платёж / Автоплатёж» — query-параметр шита выбора (#463).
 */

export const metadata: Metadata = {
  title: 'Новый платеж — Рентли',
};

export default async function PaymentNewRoutePage({
  params,
  searchParams,
}: PageProps<'/properties/[id]/payments/new'>) {
  const { id } = await params;
  const query = await searchParams;
  const draftType = parsePaymentDraftTypeParam(query.type);

  if (draftType === undefined) {
    redirect(ROUTES.propertyPayments(id));
  }

  return <PaymentCreateWizardScreen propertyId={id} draftType={draftType} />;
}
