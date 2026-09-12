'use client';

import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapGlobalPaymentFeed, mapGlobalPaymentObject, mapGlobalPaymentSearch, mapPayment, mapPaymentOperation, mapOperationsSummary } from '@/entities/payment';
import type {
  GlobalPaymentFeed,
  GlobalPaymentObject,
  GlobalPaymentSearch,
  OperationCreateCommand,
  OperationsSummary,
  PaymentCategoryView,
  Payment,
  PaymentCreateCommand,
  PaymentOperation,
  PaymentUpdateCommand,
} from '@/entities/payment';
import {
  globalOperationKeys,
  globalPaymentKeys,
  paymentKeys,
  paymentOperationKeys,
  type GlobalOperationScope,
  rentalKeys,
  type PaymentOperationOrder,
  type PaymentOperationScope,
  type PaymentOperationStatusFilter,
} from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import { flattenUniqueById } from '../lib/feed-pages';

type PaymentsResponse = components['schemas']['PaymentsResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type OperationsSummaryResponse = components['schemas']['OperationsSummaryResponse'];
type PaymentResponseDto = components['schemas']['PaymentResponse'];
type OperationResponseDto = components['schemas']['OperationResponse'];

/** Список платежей объекта — правил с флагом автоплатежа и избранным
 * (ADR 0049): без пагинации, порядок — серверный (по дате заведения).
 * search — серверный регистронезависимый подстрочный фильтр по названию
 * ('' = без фильтра; секции экрана «Платежи объекта» ищут каждую свою). */
export function usePayments(
  propertyId: string,
  search = '',
): UseQueryResult<Payment[], ApiError> {
  return useQuery({
    queryKey: paymentKeys.list(propertyId, search),
    queryFn: async () => {
      const query = search ? `?search=${encodeURIComponent(search)}` : '';
      const response = await apiClient<PaymentsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/payments${query}`,
      );
      return response.items.map(mapPayment);
    },
    enabled: Boolean(propertyId),
  });
}

/**
 * Просроченные операции объекта (секция «Просроченные операции» экрана
 * «Платежи объекта»): статус overdue вычисляет сервер по «сегодня» в TZ
 * собственника (ADR 0048), сортировка asc — старейшая просрочка первой,
 * долг разбирают по порядку накопления. Порция — серверный дефолт 50;
 * полный список с пагинацией — usePropertyOperationsPaged. search — тот же
 * серверный фильтр по названию ('' = без фильтра).
 */
export function usePropertyOverdueOperations(
  propertyId: string,
  search = '',
): UseQueryResult<PaymentOperation[], ApiError> {
  return useQuery({
    queryKey: paymentOperationKeys.overdueByProperty(propertyId, search),
    queryFn: async () => {
      const query = search ? `&search=${encodeURIComponent(search)}` : '';
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/operations?status=overdue&order=asc${query}`,
      );
      return response.items.map(mapPaymentOperation);
    },
    enabled: Boolean(propertyId),
  });
}

/**
 * Порции операций объекта для страницы «Просроченные операции»: тот же
 * контракте, что у usePropertyOverdueOperations, но с limit/offset —
 * pageParam — offset; следующая страница есть, пока порция полная.
 */
export function usePropertyOperationsPaged(
  propertyId: string,
  params: { readonly status: PaymentOperationStatusFilter; readonly order: PaymentOperationOrder },
): UseInfiniteQueryResult<ReadonlyArray<PaymentOperation>, ApiError> {
  const { status, order } = params;
  return useInfiniteQuery({
    queryKey: paymentOperationKeys.byPropertyPaged(propertyId, status, order),
    queryFn: async ({ pageParam }) => {
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/operations`
          + `?status=${status}&order=${order}`
          + `&limit=${OPERATIONS_PAGE_SIZE}&offset=${String(pageParam)}`,
      );
      return response.items.map(mapPaymentOperation);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.length < OPERATIONS_PAGE_SIZE
        ? undefined
        : allPages.length * OPERATIONS_PAGE_SIZE,
    select: (data) => data.pages.flat(),
    enabled: Boolean(propertyId),
  });
}

/**
 * Создание платежа (визард #464): контракт POST создания camelCase — команда
 * уходит телом без переупаковки; `since` проставляет сервер (сегодня в TZ
 * собственника). Инвалидация списков платежей и операций объекта.
 */
export function useCreatePayment(
  propertyId: string,
): UseMutationResult<Payment, ApiError, PaymentCreateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: PaymentCreateCommand) => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments`,
        { method: 'POST', body: JSON.stringify(command) },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/**
 * Создание ручной операции (#570, контракт POST из #569): факт рождается
 * paid с датой «сегодня владельца» — команда уходит телом без
 * переупаковки. Объект — часть переменных мутации: у глобального входа
 * он выбирается на шаге «Выбрать объект», когда хук уже смонтирован.
 * Инвалидация операций объекта (списки, просрочка, сводка) и глобальной
 * ленты со сводками.
 */
export function useCreateOperation(): UseMutationResult<
  PaymentOperation,
  ApiError,
  { readonly propertyId: string; readonly command: OperationCreateCommand }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ propertyId, command }) => {
      const response = await apiClient<OperationResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/operations`,
        { method: 'POST', body: JSON.stringify(command) },
      );
      return mapPaymentOperation(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
      void queryClient.invalidateQueries({ queryKey: globalOperationKeys.all });
    },
  });
}

/**
 * Правка платежа (#467, экран правки): частичный PATCH — команда уже
 * посчитана диффом формы (update-model), опущенное поле остаётся без
 * изменений, `since` серверный. Сервер в транзакции пересоздаёт плановое
 * вхождение и гоняет тик — инвалидируются и правила, и операции.
 */
export function useUpdatePayment(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, PaymentUpdateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: PaymentUpdateCommand) => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`,
        { method: 'PATCH', body: JSON.stringify(command) },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/**
 * Удаление платежа (только владелец, история 33 спеки #453): плановые
 * операции с датой от сегодня сносятся всегда; просроченные остаются
 * долгом (`keepOverdue: true`, безопасный дефолт модалки) или сносятся
 * вместе с правилом (`false`). Оплаченные факты неприкосновенны — в истории
 * они остаются с пометкой «платёж удалён» (`payment_id` обнуляется).
 */
export function useDeletePayment(
  propertyId: string,
  paymentId: string,
): UseMutationResult<void, ApiError, boolean> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (keepOverdue: boolean) => {
      await apiClient<void>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
          + `?keep_overdue=${keepOverdue}`,
        { method: 'DELETE' },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/**
 * Чтение платежа для страницы платежа (#465). Порции списков операций
 * отдельно — этот хук тянет только правило.
 */
export function usePayment(
  propertyId: string,
  paymentId: string,
): UseQueryResult<Payment, ApiError> {
  return useQuery({
    queryKey: paymentKeys.detail(propertyId, paymentId),
    queryFn: async () => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`,
      );
      return mapPayment(response);
    },
    enabled: Boolean(propertyId) && Boolean(paymentId),
  });
}

/**
 * Операции платежа одного статуса — вход мутации «Оплатить» (head asc =
 * старейшее неоплаченное; просроченные запрашиваются отдельным ключом и
 * имеют приоритет над плановыми) и секция «Просроченные» страницы платежа
 * (#465). Порядок asc закреплён контрактом «долг разбирают по порядку
 * накопления»; порция — серверный дефолт 50. `options.enabled` глушит запрос
 * там, где статусный список не нужен (экран правки #467 тянет просрочки
 * только для владельца — от них зависит чекбокс модалки удаления).
 */
export function usePaymentOperationsByStatus(
  propertyId: string,
  paymentId: string,
  status: PaymentOperationStatusFilter,
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<PaymentOperation[], ApiError> {
  return useQuery({
    queryKey: paymentOperationKeys.byPaymentWithStatus(propertyId, paymentId, status),
    queryFn: async () => {
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
          + `/operations?status=${status}&order=asc`,
      );
      return response.items.map(mapPaymentOperation);
    },
    enabled: (options.enabled ?? true) && Boolean(propertyId) && Boolean(paymentId),
  });
}

/** Размер порции всех списков операций (правило платформы, резолюция #452:
 * по 50 + бесконечный скролл; серверный дефолт — те же 50). */
export const OPERATIONS_PAGE_SIZE = 50;

/**
 * Порции операций платежа для подэкранов страницы (#466): «История»
 * (status=paid, порядок по чипу «Новые») и полный список просроченных
 * (status=overdue, asc — долг разбирают по порядку накопления). pageParam —
 * offset; следующая страница есть, пока порция полная. Направление — часть
 * ключа: переключение чипа читает другой кэш с первой порции.
 */
export function usePaymentOperationsPaged(
  propertyId: string,
  paymentId: string,
  params: { readonly status: PaymentOperationStatusFilter; readonly order: PaymentOperationOrder },
): UseInfiniteQueryResult<ReadonlyArray<PaymentOperation>, ApiError> {
  const { status, order } = params;
  return useInfiniteQuery({
    queryKey: paymentOperationKeys.byPaymentPaged(propertyId, paymentId, status, order),
    queryFn: async ({ pageParam }) => {
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
          + `/operations?status=${status}&order=${order}`
          + `&limit=${OPERATIONS_PAGE_SIZE}&offset=${String(pageParam)}`,
      );
      return response.items.map(mapPaymentOperation);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.length < OPERATIONS_PAGE_SIZE
        ? undefined
        : allPages.length * OPERATIONS_PAGE_SIZE,
    select: (data) => data.pages.flat(),
    enabled: Boolean(propertyId) && Boolean(paymentId),
  });
}

/**
 * Порции операций объекта для экранов «Операции объекта» (#474): тот же
 * контракт, что у usePropertyOperationsPaged, но весь скоуп (тип, категории,
 * период, поиск #476) уходит и в ключ, и в query — переключение фильтра
 * читает свой кэш с первой порции. pageParam — offset; следующая страница
 * есть, пока порция полная. `options.enabled` глушит запрос (поиск #476 не
 * стреляет, пока запрос не введён). keepPreviousData держит предыдущий
 * период на экране, пока едет новый — смена фильтра не подменяет страницу
 * скелетоном (решение владельца о плавности, #472).
 */
export function usePropertyOperationsScopedPaged(
  propertyId: string,
  scope: PaymentOperationScope,
  options: { readonly enabled?: boolean } = {},
): UseInfiniteQueryResult<ReadonlyArray<PaymentOperation>, ApiError> {
  return useInfiniteQuery({
    queryKey: paymentOperationKeys.byPropertyScopedPaged(propertyId, scope),
    queryFn: async ({ pageParam }) => {
      const params = new URLSearchParams({
        status: scope.status,
        order: scope.order,
        limit: String(OPERATIONS_PAGE_SIZE),
        offset: String(pageParam),
      });
      if (scope.type !== undefined) {
        params.set('type', scope.type);
      }
      if (scope.categories !== undefined && scope.categories.length > 0) {
        params.set('category', scope.categories.join(','));
      }
      if (scope.dateFrom !== undefined) {
        params.set('date_from', scope.dateFrom);
      }
      if (scope.dateTo !== undefined) {
        params.set('date_to', scope.dateTo);
      }
      if (scope.search !== undefined && scope.search !== '') {
        params.set('search', scope.search);
      }
      const response = await apiClient<OperationsResponse>(
        `/properties/${encodeURIComponent(propertyId)}/operations?${params.toString()}`,
      );
      return response.items.map(mapPaymentOperation);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.length < OPERATIONS_PAGE_SIZE
        ? undefined
        : allPages.length * OPERATIONS_PAGE_SIZE,
    select: (data) => data.pages.flat(),
    placeholderData: keepPreviousData,
    enabled: (options.enabled ?? true) && Boolean(propertyId),
  });
}

/**
 * Сводка периода объекта (#474) за карточками «Расходы/Доходы» и чипом
 * «Категория»: итоги всегда оба направления, разбивка — только категории с
 * операциями (по сумме убывание). Статус и период — те же, что у списка,
 * поэтому карточки и список всегда согласны друг с другом. Поиск (#476) —
 * тот же предикат, что у списка: разбивка становится чипами совпавших
 * категорий экрана поиска; `options.enabled` глушит запрос, пока запрос
 * поиска не введён; keepPreviousData — сводка не мигает при смене периода
 * или запроса (решение владельца о плавности, #472).
 */
export function usePropertyOperationsSummary(
  propertyId: string,
  scope: PaymentOperationScope,
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<OperationsSummary, ApiError> {
  return useQuery({
    queryKey: paymentOperationKeys.summary(propertyId, scope),
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set('status', scope.status);
      if (scope.type !== undefined) {
        params.set('type', scope.type);
      }
      if (scope.dateFrom !== undefined) {
        params.set('date_from', scope.dateFrom);
      }
      if (scope.dateTo !== undefined) {
        params.set('date_to', scope.dateTo);
      }
      if (scope.search !== undefined && scope.search !== '') {
        params.set('search', scope.search);
      }
      const response = await apiClient<OperationsSummaryResponse>(
        `/properties/${encodeURIComponent(propertyId)}/operations/summary?${params.toString()}`,
      );
      return mapOperationsSummary(response);
    },
    placeholderData: keepPreviousData,
    enabled: (options.enabled ?? true) && Boolean(propertyId),
  });
}

/** Скоуп глобальной ленты → общая часть query-параметров (объекты, период,
 * направление, архив, поиск); пагинация и порядок — у порции, сводка их не
 * принимает. Общее горло хуков и прогрева хабов #626. */
function operationsScopeParams(scope: GlobalOperationScope): URLSearchParams {
  const params = new URLSearchParams();
  if (scope.propertyIds !== undefined && scope.propertyIds.length > 0) {
    params.set('propertyIds', scope.propertyIds.join(','));
  }
  if (scope.categories !== undefined && scope.categories.length > 0) {
    params.set('category', scope.categories.join(','));
  }
  if (scope.type !== undefined) {
    params.set('type', scope.type);
  }
  if (scope.includeArchived) {
    params.set('includeArchived', 'true');
  }
  if (scope.dateFrom !== undefined) {
    params.set('date_from', scope.dateFrom);
  }
  if (scope.dateTo !== undefined) {
    params.set('date_to', scope.dateTo);
  }
  if (scope.search !== undefined && scope.search !== '') {
    params.set('search', scope.search);
  }
  return params;
}

/** Порция глобальной ленты (#597): строки плюс keyset-продолжение —
 * opaque-курсор следующей порции, null = лента исчерпана. */
export type GlobalOperationsPageData = {
  readonly items: ReadonlyArray<PaymentOperation>;
  readonly nextCursor: string | null;
};

/** Чистый fetch порции глобальной ленты — общее горло хука и прогрева
 * хабов #626. cursor — keyset-продолжение прошлого ответа (#597);
 * undefined читает ленту с начала. */
export async function fetchGlobalOperationsPage(
  scope: GlobalOperationScope,
  cursor?: string,
): Promise<GlobalOperationsPageData> {
  const params = operationsScopeParams(scope);
  params.set('order', scope.order);
  params.set('limit', String(OPERATIONS_PAGE_SIZE));
  if (cursor) {
    params.set('cursor', cursor);
  }
  const response = await apiClient<OperationsResponse>(`/operations?${params.toString()}`);
  return {
    items: response.items.map(mapPaymentOperation),
    nextCursor: response.nextCursor ?? null,
  };
}

/** Есть ли следующая порция: пока сервер отдал keyset-продолжение (#597). */
export function operationsNextPageParam(
  lastPage: GlobalOperationsPageData,
): string | undefined {
  return lastPage.nextCursor ?? undefined;
}

/**
 * Глобальная лента операций (карта #540, #541): платёжные факты видимой
 * книги (ADR 0028), архивные исключены сервером; лента paid-only. Порции
 * листаются keyset-курсором (#597): pageParam — курсор прошлого ответа,
 * смена queryKey начинает свежий обход с пустого курсора — sentinel не
 * наследует позицию прошлых порций. `options.enabled` глушит
 * запрос (поиск #543 не стреляет, пока запрос не введён);
 * keepPreviousData держит прежнюю страницу, пока едет новая; склейка
 * порций дедуплицируется по id — страховка от повторов на гонках.
 */
export function useGlobalOperationsPaged(
  scope: GlobalOperationScope,
  options: { readonly enabled?: boolean } = {},
): UseInfiniteQueryResult<ReadonlyArray<PaymentOperation>, ApiError> {
  return useInfiniteQuery({
    queryKey: globalOperationKeys.listPaged(scope),
    queryFn: ({ pageParam }) => fetchGlobalOperationsPage(scope, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: operationsNextPageParam,
    select: (data) => flattenUniqueById(data.pages.map((page) => page.items)),
    placeholderData: keepPreviousData,
    enabled: options.enabled ?? true,
  });
}

/** Чистый fetch сводки операций — общее горло хука и прогрева хабов #626. */
export async function fetchGlobalOperationsSummary(
  scope: GlobalOperationScope,
): Promise<OperationsSummary> {
  const params = operationsScopeParams(scope);
  const query = params.toString();
  const response = await apiClient<OperationsSummaryResponse>(
    `/operations/summary${query.length > 0 ? `?${query}` : ''}`,
  );
  return mapOperationsSummary(response);
}

/**
 * Сохранение ручного порядка избранного (карта #573, #576; кнопка
 * «Сохранить» режима правки #579): полный список видимых избранных правил
 * в новом порядке — сервер в одной транзакции строит плотные 1-based
 * позиции (полное замещение). Чистая запись порядка — ничего не тикает;
 * инвалидируется глобальный фид, из которого читают оба экрана избранного.
 */
export function useSaveFavoritesOrder(): UseMutationResult<
  void,
  ApiError,
  readonly string[]
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (paymentIds: readonly string[]) => {
      await apiClient<void>('/payments/favorites/order', {
        method: 'PUT',
        body: JSON.stringify({ paymentIds: [...paymentIds] }),
      });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: globalPaymentKeys.all });
    },
  });
}

/**
 * Глобальная сводка периода (#540) за карточками «Расходы/Доходы» и чипом
 * «Категория»: итоги всегда оба направления; категорийный фильтр сводку не
 * сужает (решение владельца #539) — в запрос уходят только объекты, период,
 * тип направления (#548) и поиск. Поиск (#543) — тот же предикат, что у
 * списка: разбивка становится чипами совпавших категорий; `options.enabled`
 * глушит запрос, keepPreviousData — карточки не мигают при смене фильтров.
 */
export function useGlobalOperationsSummary(
  scope: GlobalOperationScope,
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<OperationsSummary, ApiError> {
  return useQuery({
    queryKey: globalOperationKeys.summary(scope),
    queryFn: () => fetchGlobalOperationsSummary(scope),
    placeholderData: keepPreviousData,
    enabled: options.enabled ?? true,
  });
}

/**
 * Удаление операции (решение владельца): planned (долг) и paid получают
 * tombstone-статус cancelled — факт оплаты и долг исчезают, расписание не
 * трогается (надгробие блокирует повторную материализацию на тике).
 * Инвалидация операций и правил (isCompleted мог пересчитаться).
 */export function useDeleteOperation(
  propertyId: string,
): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (operationId: string) => {
      await apiClient<void>(
        `/properties/${encodeURIComponent(propertyId)}`
          + `/operations/${encodeURIComponent(operationId)}`,
        { method: 'DELETE' },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
    },
  });
}

/**
 * Пауза платежа (история 18 спеки #453): бессрочно с сегодняшнего дня,
 * генерация останавливается. Сервер в транзакции гоняет тик, поэтому
 * инвалидируются и правила, и операции.
 */
export function usePausePayment(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/pause`,
        { method: 'POST' },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/** Возобновление без подтверждения (история 20); расписание идёт от якорей. */
export function useResumePayment(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/resume`,
        { method: 'POST' },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}

/**
 * Атомарный toggle избранного (PUT favorite c телом `{favorite}` —
 * резолюция #452, без read-modify-write через PATCH). Правило и команда
 * приходят переменными мутации: один инстанс обслуживает и звезду
 * страницы платежа, и пачку удалений режима правки избранного (#579).
 */
export function useSetPaymentFavorite(): UseMutationResult<
  Payment,
  ApiError,
  { readonly propertyId: string; readonly paymentId: string; readonly favorite: boolean }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ propertyId, paymentId, favorite }) => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/favorite`,
        { method: 'PUT', body: JSON.stringify({ favorite }) },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      // Строка избранного живёт и в глобальном фиде (карта #573): звезда
      // на месте — фид перечитывается, строка исчезает/появляется сразу.
      void queryClient.invalidateQueries({ queryKey: globalPaymentKeys.all });
    },
  });
}

/**
 * Одна операция для страницы операции: статус приходит вычисляемым view
 * (planned/paid/overdue — overdue по TZ собственника, ADR 0048). Ключ —
 * ветка paymentOperationKeys.all, поэтому оплата инвалидирует её наравне
 * со списками.
 */
export function useOperation(
  propertyId: string,
  operationId: string,
): UseQueryResult<PaymentOperation, ApiError> {
  return useQuery({
    queryKey: paymentOperationKeys.byId(propertyId, operationId),
    queryFn: async () => {
      const response = await apiClient<components['schemas']['OperationResponse']>(
        `/properties/${encodeURIComponent(propertyId)}`
          + `/operations/${encodeURIComponent(operationId)}`,
      );
      return mapPaymentOperation(response);
    },
    enabled: Boolean(propertyId) && Boolean(operationId),
  });
}

/**
 * «Оплатить сейчас» (история 24/25): `planned → paid` у конкретного
 * вхождения, дата оплаты — серверная, расписание не сдвигается. Какая
 * операция гасится («старейшее неоплаченное»), решает виджет по спискам
 * операций; сервер подтверждает контрактной ошибкой 409 на повторную
 * оплату. Гашение операции арендной платы двигает прогресс аренды
 * (#531) — инвалидируется и срез rentals.
 */
export function usePayOperation(
  propertyId: string,
): UseMutationResult<PaymentOperation, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (operationId: string) => {
      const response = await apiClient<components['schemas']['OperationResponse']>(
        `/properties/${encodeURIComponent(propertyId)}`
          + `/operations/${encodeURIComponent(operationId)}/pay`,
        { method: 'POST' },
      );
      return mapPaymentOperation(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: rentalKeys.all });
    },
  });
}

/** Чистый fetch фида «Платежей» — общее горло хука и прогрева хабов
 * #626 (кэш прогревается тем же кодом, что читает экран). */
export async function fetchGlobalPaymentsFeed(): Promise<GlobalPaymentFeed> {
  const response = await apiClient<components['schemas']['PaymentsGlobalResponse']>(
    '/payments',
  );
  return mapGlobalPaymentFeed(response);
}

/**
 * Фид главного экрана «Платежи» (карта #573, #575): все правила видимой
 * книги целиком (пагинации нет) плюс счётчики целого скоупа — карточки
 * «Все избранные»/«Все просроченные» читают их; поиск счётчики не сужает.
 * Секции (избранные/просроченные) из фида режет виджет — в том числе
 * сортировку избранного по favoriteOrder (#576).
 */
export function useGlobalPayments(): UseQueryResult<GlobalPaymentFeed, ApiError> {
  return useQuery({
    queryKey: globalPaymentKeys.feed,
    queryFn: fetchGlobalPaymentsFeed,
  });
}

/** Чистый fetch стопок объектов «Платежей» — общее горло хука и prefetch. */
export async function fetchGlobalPaymentObjects(
  search = '',
): Promise<ReadonlyArray<GlobalPaymentObject>> {
  const query = search ? `?search=${encodeURIComponent(search)}` : '';
  const response = await apiClient<components['schemas']['PaymentObjectsGlobalResponse']>(
    `/payments/objects${query}`,
  );
  return response.items.map(mapGlobalPaymentObject);
}

/**
 * Стопки объектов для секции «Платежи объектов» и страницы «Объекты»
 * (#575, #582): видимые объекты с группами правил и флагами просрочки;
 * закреплённые (#577) сервер отдаёт первыми. search — серверный фильтр
 * по названию/адресу ('' = без фильтра); keepPreviousData держит список,
 * пока едет новый запрос. enabled=false держит запрос спящим — поиск
 * объектов (#582) включает его только при непустом запросе.
 */
export function useGlobalPaymentObjects(
  search = '',
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<ReadonlyArray<GlobalPaymentObject>, ApiError> {
  return useQuery({
    queryKey: globalPaymentKeys.objects(search),
    queryFn: () => fetchGlobalPaymentObjects(search),
    placeholderData: keepPreviousData,
    enabled: options.enabled ?? true,
  });
}

/**
 * Чипы поиска платежей (matchedCategories, GET /payments/search #575):
 * сервер считает их по всему скоупу поискового запроса, поэтому лёгкий
 * отдельный запрос с limit=1 — строки списку не нужны. Выбранный чип
 * сужает только список (useGlobalPaymentSearch) — канон сводки операций
 * #543.
 */
export function useGlobalPaymentSearchCategories(
  query: string,
  options: { readonly enabled?: boolean } = {},
): UseQueryResult<ReadonlyArray<PaymentCategoryView>, ApiError> {
  return useQuery({
    queryKey: globalPaymentKeys.searchCategories(query),
    queryFn: async () => {
      const suffix = query ? `?search=${encodeURIComponent(query)}&limit=1` : '?limit=1';
      const response = await apiClient<components['schemas']['PaymentsSearchGlobalResponse']>(
        `/payments/search${suffix}`,
      );
      return mapGlobalPaymentSearch(response).matchedCategories;
    },
    placeholderData: keepPreviousData,
    enabled: options.enabled ?? true,
  });
}

/** Порция поиска платежей (правило платформы #452): 50 строк на страницу,
 * догрузка при скролле; серверный максимум — те же 100. */
export const PAYMENT_SEARCH_PAGE_SIZE = 50;

/**
 * Поиск глобальных платежей (GET /payments/search, #575; экран #581):
 * бесконечный запрос порциями по PAYMENT_SEARCH_PAGE_SIZE; фильтр чипа
 * (категория; направление из контракта чипов убрано — #602) сужает список
 * серверно.
 * Порции листаются keyset-курсором (#597): pageParam — nextCursor прошлого
 * ответа, смена queryKey начинает свежий обход с пустым курсором —
 * sentinel не наследует позицию прошлых порций при правке запроса.
 * matchedCategories сервер считает по всему скоупу запроса, поэтому в
 * результате они берутся с первой страницы; склейка порций дедуплицируется
 * по id — страховка от повторов на гонках.
 * keepPreviousData держит прежнюю выдачу, пока едет новый запрос (правка
 * запроса не мигает); пустой запрос экран не выполняет.
 */
export function useGlobalPaymentSearch(
  query: string,
  filter: { readonly category?: string } = {},
  options: { readonly enabled?: boolean } = {},
): UseInfiniteQueryResult<GlobalPaymentSearch, ApiError> {
  const { category = '' } = filter;
  return useInfiniteQuery({
    queryKey: globalPaymentKeys.search(query, category),
    queryFn: async ({ pageParam }) => {
      const params = new URLSearchParams();
      if (query) params.set('search', query);
      if (category) params.set('category', category);
      params.set('limit', String(PAYMENT_SEARCH_PAGE_SIZE));
      if (pageParam) params.set('cursor', pageParam);
      const response = await apiClient<components['schemas']['PaymentsSearchGlobalResponse']>(
        `/payments/search?${params.toString()}`,
      );
      return mapGlobalPaymentSearch(response);
    },
    initialPageParam: '',
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    select: (data): GlobalPaymentSearch => ({
      items: flattenUniqueById(data.pages.map((page) => page.items)),
      matchedCategories: data.pages[0]?.matchedCategories ?? [],
      nextCursor: data.pages[data.pages.length - 1]?.nextCursor ?? null,
    }),
    placeholderData: keepPreviousData,
    enabled: options.enabled ?? true,
  });
}
