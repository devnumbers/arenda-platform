import { redirect } from 'next/navigation';
import { ROUTES } from '@/shared/config/routes';

export default async function SubscriptionPaymentFailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  redirect(`${ROUTES.profilePaymentDetail(id)}?payment=fail`);
}
