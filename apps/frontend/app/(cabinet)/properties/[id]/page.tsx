import type { Metadata } from 'next';
import { Suspense } from 'react';
import { PropertyDetailPage } from '@/widgets/property-detail';
import { PropertyDetailLoading } from '@/widgets/property-detail/ui/PropertyDetailLoading';

type PropertyDetailRoutePageProps = {
  params: Promise<{ id: string }>;
};

export const metadata: Metadata = {
  title: 'Мой объект — Arenda Platform',
  description: 'Просмотр объекта недвижимости',
};

export default async function PropertyDetailRoutePage({
  params,
}: PropertyDetailRoutePageProps) {
  await params;

  return (
    <Suspense fallback={<PropertyDetailLoading />}>
      <PropertyDetailPage />
    </Suspense>
  );
}
