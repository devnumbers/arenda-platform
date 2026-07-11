'use client';

import {
  useQuery,
  useMutation,
  useQueryClient,
  type UseQueryResult,
  type UseMutationResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import type { components } from '@/shared/api/generated';
import { type TariffName } from '@/entities/user/model/types';
import {
  mapAddPaymentMethodResponse,
  mapChangeTariffResponse,
  mapPaymentMethodResponse,
  mapSubscriptionPaymentResponse,
  mapSubscriptionResponse,
  mapTariffResponse,
} from '@/entities/billing/model/mappers';
import type {
  AddPaymentMethodResult,
  ChangeTariffResult,
  PaymentMethod,
  Subscription,
  SubscriptionPayment,
  Tariff,
} from '@/entities/billing/model/types';
import { billingKeys } from './keys';

type AutoRenewRequest = components['schemas']['AutoRenewRequest'];
type ChangeTariffRequest = Omit<
  components['schemas']['ChangeTariffRequest'],
  'tariffName'
> & {
  tariffName: TariffName;
};
type AddPaymentMethodRequest = components['schemas']['AddPaymentMethodRequest'];
type SubscriptionPaymentsResponse =
  components['schemas']['SubscriptionPaymentsResponse'];

/** Pending-платёж старше этого возраста считается «зависшим» — UI меняет текст. */
export const PAYMENT_STALE_MS = 15 * 60 * 1000;

const PAYMENT_POLL_INTERVAL_MS = 5000;
const PAYMENT_POLL_MAX_AGE_MS = 30 * 60 * 1000;

/** Polling продолжается, пока есть pending-платёж не старше PAYMENT_POLL_MAX_AGE_MS. */
function hasRecentPendingPayment(
  data: SubscriptionPaymentsResponse | undefined,
  paymentId?: string,
): boolean {
  if (!data) return false;
  return data.items.some((item) => {
    if (item.status !== 'pending') return false;
    if (paymentId && item.id !== paymentId) return false;
    const ageMs = Date.now() - new Date(item.createdAt).getTime();
    return ageMs < PAYMENT_POLL_MAX_AGE_MS;
  });
}

export function useTariffs(): UseQueryResult<Tariff[], ApiError> {
  return useQuery({
    queryKey: billingKeys.tariffs,
    queryFn: () => apiClient<components['schemas']['TariffsResponse']>('/tariffs'),
    select: (data) => data.items.map(mapTariffResponse),
  });
}

export function useSubscription(): UseQueryResult<Subscription, ApiError> {
  return useQuery({
    queryKey: billingKeys.subscription,
    queryFn: () => apiClient<components['schemas']['Subscription']>('/subscription'),
    select: mapSubscriptionResponse,
  });
}

export function useToggleAutoRenew(): UseMutationResult<
  void,
  ApiError,
  AutoRenewRequest
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ enabled }: AutoRenewRequest) => {
      await apiClient<void>('/subscription/auto-renew', {
        method: 'PATCH',
        body: JSON.stringify({ enabled }),
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useCancelSubscription(): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      await apiClient<void>('/subscription/cancel', { method: 'POST' });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useChangeTariff(): UseMutationResult<
  ChangeTariffResult,
  ApiError,
  ChangeTariffRequest
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ tariffName, period }: ChangeTariffRequest) => {
      const response = await apiClient<components['schemas']['ChangeTariffResponse']>(
        '/subscription/change',
        {
          method: 'POST',
          body: JSON.stringify({ tariffName, period }),
        },
      );
      return mapChangeTariffResponse(response);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function usePaymentMethods(): UseQueryResult<PaymentMethod[], ApiError> {
  return useQuery({
    queryKey: billingKeys.paymentMethods,
    queryFn: () =>
      apiClient<components['schemas']['PaymentMethodsResponse']>('/subscription/payment-methods'),
    select: (data) => data.items.map(mapPaymentMethodResponse),
  });
}

export function useAddPaymentMethod(): UseMutationResult<
  AddPaymentMethodResult,
  ApiError,
  AddPaymentMethodRequest
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (body: AddPaymentMethodRequest) => {
      const response = await apiClient<components['schemas']['AddPaymentMethodResponse']>(
        '/subscription/payment-methods',
        {
          method: 'POST',
          body: JSON.stringify(body),
        },
      );
      return mapAddPaymentMethodResponse(response);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
    },
  });
}

export function useActivatePaymentMethod(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient<void>(
        `/subscription/payment-methods/${encodeURIComponent(id)}/activate`,
        { method: 'POST' },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
      queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useSyncPaymentMethods(): UseMutationResult<
  components['schemas']['PaymentMethodsResponse'],
  ApiError,
  void
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () =>
      apiClient<components['schemas']['PaymentMethodsResponse']>(
        '/subscription/payment-methods/sync',
        { method: 'POST' },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
      queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useDeletePaymentMethod(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient<void>(
        `/subscription/payment-methods/${encodeURIComponent(id)}`,
        { method: 'DELETE' },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: billingKeys.paymentMethods });
      queryClient.invalidateQueries({ queryKey: billingKeys.subscription });
    },
  });
}

export function useSubscriptionPayments(): UseQueryResult<
  SubscriptionPayment[],
  ApiError
> {
  return useQuery({
    queryKey: billingKeys.payments,
    queryFn: () =>
      apiClient<components['schemas']['SubscriptionPaymentsResponse']>('/subscription/payments'),
    select: (data) => data.items.map(mapSubscriptionPaymentResponse),
  });
}

export function usePendingPayment(): UseQueryResult<
  SubscriptionPayment | undefined,
  ApiError
> {
  return useQuery({
    queryKey: billingKeys.payments,
    queryFn: () =>
      apiClient<components['schemas']['SubscriptionPaymentsResponse']>('/subscription/payments'),
    select: (data) =>
      data.items
        .map(mapSubscriptionPaymentResponse)
        .filter((payment) => payment.status === 'pending')
        .sort(
          (a, b) =>
            new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
        )[0],
    refetchInterval: (query) =>
      hasRecentPendingPayment(query.state.data)
        ? PAYMENT_POLL_INTERVAL_MS
        : false,
  });
}

export function useSubscriptionPayment(
  id: string,
  enabled = true,
): UseQueryResult<SubscriptionPayment | undefined, ApiError> {
  return useQuery({
    queryKey: billingKeys.payment(id),
    queryFn: () =>
      apiClient<components['schemas']['SubscriptionPaymentsResponse']>('/subscription/payments'),
    select: (data) =>
      data.items
        .map(mapSubscriptionPaymentResponse)
        .find((payment) => payment.id === id),
    enabled,
    refetchInterval: (query) =>
      hasRecentPendingPayment(query.state.data, id)
        ? PAYMENT_POLL_INTERVAL_MS
        : false,
  });
}
