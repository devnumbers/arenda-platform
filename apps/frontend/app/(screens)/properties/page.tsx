import type {Metadata} from 'next';
import {PropertiesPage} from '@/widgets/properties';
import {parseFiltersFromParams, parseSortFromParams} from '@/widgets/properties';

export const metadata: Metadata = {
    title: 'Объекты — Рентли',
    description: 'Список объектов',
};

type PropertiesRoutePageProps = {
    readonly searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function PropertiesRoutePage({searchParams}: PropertiesRoutePageProps) {
    const resolved = searchParams ? await searchParams : {};
    const initialFilters = parseFiltersFromParams(resolved);
    const initialSort = parseSortFromParams(resolved);

    return <PropertiesPage initialFilters={initialFilters} initialSort={initialSort}/>;
}
