import type {Metadata} from 'next';
import {PropertiesPage} from '@/widgets/properties';
import {parseSortFromParams} from '@/widgets/properties';

export const metadata: Metadata = {
    title: 'Объекты — Рентли',
    description: 'Список объектов',
};

export default async function PropertiesRoutePage({
    searchParams,
}: PageProps<'/properties'>) {
    const resolved = await searchParams;
    const initialSort = parseSortFromParams(resolved);

    return <PropertiesPage initialSort={initialSort}/>;
}
