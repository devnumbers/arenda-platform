'use client';

import { useEffect, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { ROUTES } from '@/shared/config/routes';
import { type GlobalOperationScope } from '@/shared/api/query-keys';
import {
  globalOperationsPagedQueryOptions,
  globalOperationsSummaryQueryOptions,
  globalPaymentObjectsQueryOptions,
  globalPaymentsFeedQueryOptions,
} from '@/features/payments';
import { globalTasksQueryOptions } from '@/features/tasks';
import { contactBookQuery } from '@/features/contacts';
import { propertiesListQueryOptions } from '@/features/properties';

/**
 * Прогрев верхнеуровневых данных хабов на маунте оболочки (#626, карта
 * #603): переходы в разделы открываются с данными сразу, как при повторном
 * входе, — на клике остаётся 0 блокирующих API-запросов (замер: холодный
 * вход в хаб на 4G 0,8–1,8 с против 15–25 мс тёплого). Роутер-префиксы
 * прогреваются тем же заходом: RSC-полезная нагрузка хаба приезжает до
 * перехода.
 *
 * Прогрев — один раз на маунт оболочки, хабы по кругу с паузой, чтобы не
 * занимать сеть залпом. Повторный прогрев не нужен: prefetchQuery уважает
 * staleTime 30 с из QueryProvider и не перечитывает свежее.
 *
 * Данные — те же query-ключи реестра и те же fetch-функции, что читают
 * экраны (api-слой фич), поэтому кэш, инвалидации и дедупликация
 * react-query работают без исключений. Реестр — только верхнеуровневые
 * списки хабов в дефолтном срезе (без фильтров); детали и отфильтрованные
 * срезы не прогреваются — не угадываем вместо пользователя.
 */
interface HubPrefetchEntry {
  readonly prefix: string;
  readonly prefetch: (client: QueryClient) => void;
}

/** Реестр хабов: навигационный префикс → прогрев верхнеуровневых данных. */
const HUB_ENTRIES: ReadonlyArray<HubPrefetchEntry> = [
  {
    prefix: ROUTES.properties,
    prefetch: (client) => {
      void client.prefetchQuery(propertiesListQueryOptions());
    },
  },
  {
    prefix: ROUTES.payments,
    prefetch: (client) => {
      void client.prefetchQuery(globalPaymentsFeedQueryOptions());
      void client.prefetchQuery(globalPaymentObjectsQueryOptions());
    },
  },
  {
    prefix: ROUTES.operations,
    // Дефолтный срез ленты — весь период (#670), все объекты, без
    // категорий: те же ключи, что читает экран без применённого периода.
    // Сводка без периода — она же all-time, гейт «Операций еще не было»
    // на экране в дефолтном состоянии.
    prefetch: (client) => {
      const scope: GlobalOperationScope = { order: 'desc' };
      void client.prefetchInfiniteQuery(globalOperationsPagedQueryOptions({ scope }));
      void client.prefetchQuery(globalOperationsSummaryQueryOptions({ scope }));
    },
  },
  {
    prefix: ROUTES.tasks,
    // Бакеты без фильтра — дефолт EMPTY_TASKS_FEED_FILTER экрана ленты.
    prefetch: (client) => {
      void client.prefetchQuery(globalTasksQueryOptions({
        completed: false,
        propertyIds: [],
        withoutProperty: false,
      }));
      void client.prefetchQuery(globalTasksQueryOptions({
        completed: true,
        propertyIds: [],
        withoutProperty: false,
      }));
      void client.prefetchQuery(propertiesListQueryOptions());
    },
  },
  {
    prefix: ROUTES.contacts,
    // Дефолтная порция книги — тот же ключ и тот же fetch, что читает
    // useContactBook на экране (sort/order экрана живут в URL; прогрет
    // только дефолт «Имя ↑»); первая порция keyset-обхода (#600).
    prefetch: (client) => {
      void client.prefetchInfiniteQuery(contactBookQuery());
    },
  },
];

/** Пауза между прогревами хабов на маунте — не занимаем сеть залпом. */
const WARMUP_SPACING_MS = 400;

export function HubPrefetchProvider({ children }: { readonly children: ReactNode }): JSX.Element {
  const queryClient = useQueryClient();
  const router = useRouter();

  useEffect(() => {
    let cancelled = false;
    const warmup = async (): Promise<void> => {
      for (const entry of HUB_ENTRIES) {
        if (cancelled) return;
        router.prefetch(entry.prefix);
        entry.prefetch(queryClient);
        await new Promise((resolve) => setTimeout(resolve, WARMUP_SPACING_MS));
      }
    };
    void warmup();
    return () => {
      cancelled = true;
    };
  }, [queryClient, router]);

  return <>{children}</>;
}
