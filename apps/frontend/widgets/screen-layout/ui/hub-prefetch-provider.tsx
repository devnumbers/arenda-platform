'use client';

import { useCallback, useEffect, useRef, useState, type JSX, type ReactNode } from 'react';
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
import { NavIntentProvider } from '@/shared/ui/design';
import {
  matchHubPrefix,
  createPrefetchThrottle,
  readPrefetchStrategy,
  type PrefetchStrategy,
} from '../lib/hub-prefetch-model';

/**
 * Prefetch-прототип #610 (исследование карты #603): навигационный хром
 * сообщает интент (hover/pointerdown/focus), а прогретые react-query кэши
 * делают переходы в разделы мгновенными — экран сразу показывает данные,
 * как при повторном входе (замер «до»: тёплый вход 15–25 мс против
 * 0,8–1,8 с холодного на 4G).
 *
 * Два триггера (переключатель ?prefetch=off|intent|warm|all; по умолчанию
 * off — прототип спит до решения владельца о внедрении, тикет внедрения
 * отдельный):
 *  - intent: по интенту ссылки хаба — router.prefetch (RSC/чанки) плюс
 *    prefetchQuery верхнеуровневых данных хаба, не чаще раза в 5 с на хаб;
 *  - warm: на маунте оболочки — все хабы по кругу с паузой, один раз за
 *    сессию страницы (повторный прогрев не нужен: prefetchQuery уважает
 *    staleTime 30 с из QueryProvider и не перечитывает свежее).
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
        initialPageParam: 0,
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
/** Окно троттлинга интента на хаб. */
const INTENT_THROTTLE_MS = 5000;

/** Читает стратегию исследования из адреса (только на клиенте). Прямое
 * чтение location.search осознанно: значение нужно один раз на маунте
 * оболочки, реактивность на смену ?prefetch= не нужна. */
function readStrategy(): PrefetchStrategy {
  if (typeof window === 'undefined') return 'off';
  const value = new URLSearchParams(window.location.search).get('prefetch');
  return readPrefetchStrategy(value);
}

export function HubPrefetchProvider({ children }: { readonly children: ReactNode }): JSX.Element {
  const queryClient = useQueryClient();
  const router = useRouter();
  const [strategy] = useState(readStrategy);
  const intentThrottle = useRef(createPrefetchThrottle(INTENT_THROTTLE_MS));

  // Ручная стабилизация идентичности — не для рендера, а для контрактов
  // зависимостей: onIntent уезжает в value контекста (NavIntentProvider),
  // prefetchHub — в deps эффекта прогрева; пересоздание между рендерами
  // меняло бы контекст и перезапускало прогрев.
  const prefetchHub = useCallback(
    (entry: HubPrefetchEntry) => {
      router.prefetch(entry.prefix);
      entry.prefetch(queryClient);
    },
    [queryClient, router],
  );

  const onIntent = useCallback(
    (href: string) => {
      if (strategy === 'off' || strategy === 'warm') return;
      const entry = matchHubPrefix(HUB_ENTRIES, href);
      if (entry === undefined) {
        // Не-хабовые ссылки (уведомления, поддержка): только чанки/RSC.
        router.prefetch(href);
        return;
      }
      if (!intentThrottle.current.shouldRun(entry.prefix, Date.now())) return;
      prefetchHub(entry);
    },
    [prefetchHub, router, strategy],
  );

  useEffect(() => {
    if (strategy === 'off' || strategy === 'intent') return;
    let cancelled = false;
    const warmup = async (): Promise<void> => {
      for (const entry of HUB_ENTRIES) {
        if (cancelled) return;
        prefetchHub(entry);
        await new Promise((resolve) => setTimeout(resolve, WARMUP_SPACING_MS));
      }
    };
    void warmup();
    return () => {
      cancelled = true;
    };
  }, [prefetchHub, strategy]);

  return <NavIntentProvider onIntent={onIntent}>{children}</NavIntentProvider>;
}
