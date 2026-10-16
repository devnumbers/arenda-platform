import { Suspense } from 'react';
import type { Metadata } from 'next';
import { PropertiesHubLoading, PropertiesPage, parseSortFromParams } from '@/widgets/properties';
import { propertiesListQueryOptions } from '@/features/properties';
import { subscriptionQueryOptions } from '@/features/subscription';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

export const metadata: Metadata = {
    title: 'Объекты — Рентли',
    description: 'Список объектов',
};

/**
 * Хаб «Объекты» — серверный префетч холодного входа (§15, тикет #1422):
 * граница внутри Suspense — hover-префетч динамики рендерит RSC только до
 * неё. Подписка — второй запрос первого кадра (кнопки создания: canAdd,
 * isActionLoading); список и meta-проекции suspended_shared/today — один
 * ключ /properties (fetchProperties кладёт полный ответ, select экрана
 * проецирует строки).
 */
export default async function PropertiesRoutePage({
    searchParams,
}: PageProps<'/properties'>) {
    const resolved = await searchParams;
    const initialSort = parseSortFromParams(resolved);

    // Полный кадр хаба (план 044): хром вне фазы загрузки (§7),
    // сегментный loading.tsx снесён — его архетип стримился первым
    // кадром и детям (/properties/[id] получал скелетон списка).
    return (
        <Suspense fallback={<PropertiesHubLoading />}>
            <ServerPrefetchBoundary
                prefetch={(queryClient) => {
                    void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
                    void queryClient.prefetchQuery(subscriptionQueryOptions(serverApiClient));
                }}
            >
                <PropertiesPage initialSort={initialSort} />
            </ServerPrefetchBoundary>
        </Suspense>
    );
}
