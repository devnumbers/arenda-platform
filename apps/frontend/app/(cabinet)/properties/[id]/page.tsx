import type {Metadata} from 'next';
import {PropertyDetailPage} from '@/widgets/property-detail';

type PropertyDetailRoutePageProps = {
    params: Promise<{ id: string }>;
};

export const metadata: Metadata = {
    title: 'Мой объект — Рентли',
    description: 'Просмотр объекта недвижимости',
};

export default async function PropertyDetailRoutePage({
    params,
}: PropertyDetailRoutePageProps) {
    await params;

    return <PropertyDetailPage/>;
}
