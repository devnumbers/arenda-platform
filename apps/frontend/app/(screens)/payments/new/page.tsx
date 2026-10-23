import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';
import { parsePaymentDraftTypeParam } from '@/features/payments';
import { PaymentsGlobalCreateWizardScreen } from '@/widgets/payments';

/**
 * Визард создания платежа — глобальный вход (решение владельца 23.10.2026,
 * аналог /operations/new #570): шаги на одном маршруте, тип «Платёж /
 * Автоплатёж» — query-параметр модалки выбора хаба, объект выбирается
 * шагом «Выбрать объект».
 */

export const metadata: Metadata = {
  title: 'Новый платеж — Рентли',
};

export default async function PaymentNewRoutePage({
  searchParams,
}: PageProps<'/payments/new'>) {
  const query = await searchParams;
  const draftType = parsePaymentDraftTypeParam(query.type);

  if (draftType === undefined) {
    redirect(ROUTES.payments);
  }

  return <PaymentsGlobalCreateWizardScreen draftType={draftType} />;
}
