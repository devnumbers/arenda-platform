import type {Metadata} from 'next';
import {PropertyAboutScreen} from '@/widgets/property-detail';

export const metadata: Metadata = {
    title: 'Об объекте — Рентли',
    description: 'Данные и характеристики объекта недвижимости',
};

export default async function PropertyAboutRoutePage({
    params,
}: PageProps<'/properties/[id]/about'>) {
    await params;

    return <PropertyAboutScreen/>;
}
