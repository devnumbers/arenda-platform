import { Suspense } from 'react';
import type { Metadata } from 'next';
import { TariffChangeSuccess } from '@/widgets/profile';

export const metadata: Metadata = {
  title: 'Тариф изменён — Рентли',
  description: 'Подтверждение смены тарифа',
};

export default function TariffChangeSuccessPage() {
  return (
    <Suspense fallback={null}>
      <TariffChangeSuccess />
    </Suspense>
  );
}
