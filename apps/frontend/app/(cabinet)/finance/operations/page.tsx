import type {Metadata} from 'next';
import {OperationsPage} from '@/widgets/operations/ui/OperationsPage';
import {parseOperationsFromParams} from '@/widgets/operations/lib/parse-operation-search-params';

export const metadata: Metadata = {
    title: 'Операции — Рентли',
    description: 'Список финансовых операций',
};

type PageProps = {
    readonly searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function FinanceOperationsPage({searchParams}: PageProps) {
    const resolved = searchParams ? await searchParams : {};
    const initial = parseOperationsFromParams(resolved);

    return <OperationsPage initial={initial}/>;
}
