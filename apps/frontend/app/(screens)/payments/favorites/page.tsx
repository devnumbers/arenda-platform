import type { JSX } from 'react';
import type { Metadata } from 'next';
import { PaymentFavoritesScreen } from '@/widgets/payments';

export const metadata: Metadata = { title: 'Избранные платежи — Рентли' };

export default function PaymentFavoritesRoutePage(): JSX.Element {
  return <PaymentFavoritesScreen />;
}
