import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { PaymentCreateWizardScreen } from '@/widgets/payments';

/**
 * Визард создания платежа (#464): один маршрут, шаги — клиентское состояние,
 * тип «Платёж / Автоплатёж» — query-параметр шита выбора (#463).
 */

export const metadata: Metadata = {
  title: 'Новый платеж — Рентли',
};

type PaymentNewRoutePageProps = {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
};

const DRAFT_TYPES = new Set(['payment', 'autopayment']);

export default async function PaymentNewRoutePage({
  params,
  searchParams,
}: PaymentNewRoutePageProps) {
  const { id } = await params;
  const query = await searchParams;
  const typeParam = typeof query.type === 'string' ? query.type : undefined;

  if (typeParam === undefined || !DRAFT_TYPES.has(typeParam)) {
    redirect(ROUTES.propertyPayments(id));
  }

  return (
    <PaymentCreateWizardScreen
      propertyId={id}
      draftType={typeParam as 'payment' | 'autopayment'}
    />
  );
}
