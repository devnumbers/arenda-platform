import type {Metadata} from 'next';
import {PropertyAboutScreen} from '@/widgets/property-detail';

type PropertyAboutRoutePageProps = {
    params: Promise<{ id: string }>;
};

export const metadata: Metadata = {
    title: 'Об объекте — Рентли',
    description: 'Данные и характеристики объекта недвижимости',
};

export default async function PropertyAboutRoutePage({
    params,
}: PropertyAboutRoutePageProps) {
    await params;

    return <PropertyAboutScreen/>;
}
