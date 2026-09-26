import type {Metadata} from 'next';
import {PropertyDetailPage} from '@/widgets/property-detail';

export const metadata: Metadata = {
    title: 'Объект — Рентли',
    description: 'Просмотр объекта недвижимости',
};

export default async function PropertyDetailRoutePage({
    params,
}: PageProps<'/properties/[id]'>) {
    await params;

    return <PropertyDetailPage/>;
}
