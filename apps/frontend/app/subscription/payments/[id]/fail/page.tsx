import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';

export default async function SubscriptionPaymentFailPage({
  params,
}: PageProps<'/subscription/payments/[id]/fail'>) {
  const { id } = await params;
  redirect(`${ROUTES.profilePaymentDetail(id)}?payment=fail`);
}
