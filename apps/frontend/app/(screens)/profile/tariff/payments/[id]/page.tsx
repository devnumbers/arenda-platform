import type { Metadata } from 'next';
import { PaymentDetail } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Платёж — Рентли',
  description: 'Детали платежа по подписке',
};

/** Каркас (SubScreenShell) собирает сам PaymentDetail: заголовок шапки —
 * дата-время платежа, он известен только после загрузки (#624). */
export default async function PaymentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return <PaymentDetail id={id} />;
}
