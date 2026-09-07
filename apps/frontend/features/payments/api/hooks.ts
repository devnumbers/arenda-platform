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
import { mapPayment, mapPaymentOperation, mapOperationsSummary } from '@/entities/payment';
import type {
  OperationsSummary,
  Payment,
  PaymentCreateCommand,
  PaymentFavoriteCommand,
  PaymentOperation,
  PaymentUpdateCommand,
} from '@/entities/payment';
import {
  globalOperationKeys,
  paymentKeys,
  paymentOperationKeys,
  type GlobalOperationScope,
  type PaymentOperationOrder,
  type PaymentOperationScope,
  type PaymentOperationStatusFilter,
} from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type PaymentsResponse = components['schemas']['PaymentsResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type OperationsSummaryResponse = components['schemas']['OperationsSummaryResponse'];
type PaymentResponseDto = components['schemas']['PaymentResponse'];

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

/**
 * Порции глобальной ленты операций (#541, контракт /operations #540):
 * платёжные факты всех видимых объектов — свои плюс с активным членством
 * (ADR 0028), архивные исключены сервером; лента paid-only. Тот же
 * пагинационный контракт, что у объектных списков: pageParam — offset,
 * следующая страница есть, пока порция полная. `options.enabled` глушит
 * запрос (поиск #543 не стреляет, пока запрос не введён);
 * keepPreviousData держит прежнюю страницу, пока едет новая.
 */
export function useGlobalOperationsPaged(
  scope: GlobalOperationScope,
  options: { readonly enabled?: boolean } = {},
): UseInfiniteQueryResult<ReadonlyArray<PaymentOperation>, ApiError> {
  return useInfiniteQuery({
    queryKey: globalOperationKeys.listPaged(scope),
    queryFn: async ({ pageParam }) => {
      const params = new URLSearchParams({
        order: scope.order,
        limit: String(OPERATIONS_PAGE_SIZE),
        offset: String(pageParam),
      });
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
      const response = await apiClient<OperationsResponse>(`/operations?${params.toString()}`);
      return response.items.map(mapPaymentOperation);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.length < OPERATIONS_PAGE_SIZE
        ? undefined
        : allPages.length * OPERATIONS_PAGE_SIZE,
    select: (data) => data.pages.flat(),
    placeholderData: keepPreviousData,
    enabled: options.enabled ?? true,
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
    queryFn: async () => {
      const params = new URLSearchParams();
      if (scope.propertyIds !== undefined && scope.propertyIds.length > 0) {
        params.set('propertyIds', scope.propertyIds.join(','));
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
      const query = params.toString();
      const response = await apiClient<OperationsSummaryResponse>(
        `/operations/summary${query.length > 0 ? `?${query}` : ''}`,
      );
      return mapOperationsSummary(response);
    },
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
 * Атомарный toggle избранного (PUT favorite c телом `{favorite}`,
 * резолюция #452) — без read-modify-write через PATCH.
 */
export function useSetPaymentFavorite(
  propertyId: string,
  paymentId: string,
): UseMutationResult<Payment, ApiError, PaymentFavoriteCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: PaymentFavoriteCommand) => {
      const response = await apiClient<PaymentResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}/favorite`,
        { method: 'PUT', body: JSON.stringify(command) },
      );
      return mapPayment(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
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
 * оплату.
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
    },
  });
}
