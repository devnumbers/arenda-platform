import type { JSX } from 'react';
import type { Metadata } from 'next';
import { PaymentsGlobalScreen } from '@/widgets/payments';

export const metadata: Metadata = { title: 'Платежи — Рентли' };

export default function PaymentsRoutePage(): JSX.Element {
  return <PaymentsGlobalScreen />;
}
