import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';

export default async function SubscriptionPaymentSuccessPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  redirect(`${ROUTES.profileTariffChangeSuccess}?paymentId=${encodeURIComponent(id)}`);
}
