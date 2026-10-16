import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';

export default async function SubscriptionPaymentSuccessPage({
  params,
}: PageProps<'/subscription/payments/[id]/success'>) {
  const { id } = await params;
  redirect(`${ROUTES.profileTariffChangeSuccess}?paymentId=${encodeURIComponent(id)}`);
}
