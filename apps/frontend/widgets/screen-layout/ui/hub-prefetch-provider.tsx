'use client';

import { useEffect, type JSX, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { ROUTES } from '@/shared/config/routes';
import {
  globalOperationKeys,
  globalPaymentKeys,
  contactKeys,
  propertyKeys,
  taskKeys,
  type GlobalOperationScope,
} from '@/shared/api/query-keys';
import {
  defaultOperationsPeriod,
  fetchGlobalOperationsPage,
  fetchGlobalOperationsSummary,
  fetchGlobalPaymentObjects,
  fetchGlobalPaymentsFeed,
  operationsNextPageParam,
} from '@/features/payments';
import { fetchGlobalTasks } from '@/features/tasks';
import { fetchContactBook } from '@/features/contacts';
import { fetchProperties } from '@/features/properties';
import { clientTodayIso } from '@/entities/payment';

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

/** Скоуп операций хаба по умолчанию: текущий месяц, все объекты, без
 * категорий (тот же расчёт периода, что на экране). */
function defaultOperationsScope(today: string): GlobalOperationScope {
  const period = defaultOperationsPeriod(today);
  return {
    order: 'desc',
    dateFrom: period.from,
    dateTo: period.to,
  };
}

/** Реестр хабов: навигационный префикс → прогрев верхнеуровневых данных. */
const HUB_ENTRIES: ReadonlyArray<HubPrefetchEntry> = [
  {
    prefix: ROUTES.properties,
    prefetch: (client) => {
      void client.prefetchQuery({ queryKey: propertyKeys.list, queryFn: fetchProperties });
    },
  },
  {
    prefix: ROUTES.payments,
    prefetch: (client) => {
      void client.prefetchQuery({
        queryKey: globalPaymentKeys.feed,
        queryFn: fetchGlobalPaymentsFeed,
      });
      void client.prefetchQuery({
        queryKey: globalPaymentKeys.objects(''),
        queryFn: () => fetchGlobalPaymentObjects(''),
      });
    },
  },
  {
    prefix: ROUTES.operations,
    prefetch: (client) => {
      const scope = defaultOperationsScope(clientTodayIso());
      void client.prefetchInfiniteQuery({
        queryKey: globalOperationKeys.listPaged(scope),
        queryFn: ({ pageParam }) => fetchGlobalOperationsPage(scope, pageParam),
        initialPageParam: undefined as string | undefined,
        getNextPageParam: operationsNextPageParam,
      });
      void client.prefetchQuery({
        queryKey: globalOperationKeys.summary(scope),
        queryFn: () => fetchGlobalOperationsSummary(scope),
      });
      // All-time сводка — гейт «Операций еще не было» на экране.
      void client.prefetchQuery({
        queryKey: globalOperationKeys.summary({ order: 'desc' }),
        queryFn: () => fetchGlobalOperationsSummary({ order: 'desc' }),
      });
    },
  },
  {
    prefix: ROUTES.tasks,
    // Бакеты без фильтра — дефолт EMPTY_TASKS_FEED_FILTER экрана ленты.
    prefetch: (client) => {
      void client.prefetchQuery({
        queryKey: taskKeys.global(false, [], false),
        queryFn: () => fetchGlobalTasks(false, [], false),
      });
      void client.prefetchQuery({
        queryKey: taskKeys.global(true, [], false),
        queryFn: () => fetchGlobalTasks(true, [], false),
      });
      void client.prefetchQuery({ queryKey: propertyKeys.list, queryFn: fetchProperties });
    },
  },
  {
    prefix: ROUTES.contacts,
    // Дефолтный срез книги — тот же, что читает useContactBook на экране
    // (sort/order экрана живут в URL; прогрет только дефолт «Имя ↑»).
    prefetch: (client) => {
      void client.prefetchQuery({
        queryKey: contactKeys.list(null, '', 'name', 'asc'),
        queryFn: () => fetchContactBook(),
      });
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
