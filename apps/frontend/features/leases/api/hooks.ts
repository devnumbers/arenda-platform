'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { financeKeys, leaseKeys, operationKeys, propertyKeys } from '@/shared/api/query-keys';
import { mapLeaseResponse } from '@/entities/lease';
import type { Lease, LeaseCreateRequest, LeaseUpdateRequest } from '@/entities/lease';
import type { components } from '@/shared/api/dto';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeasesResponse = components['schemas']['LeasesResponse'];
type LeaseCreateWireRequest = components['schemas']['LeaseCreateRequest'];
type LeaseUpdateWireRequest = components['schemas']['LeaseUpdateRequest'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

// Команды приходят из виджетов в camelCase; wire-формат (snake_case) живёт
// только внутри этого модуля.
export function toCreateWireRequest(data: LeaseCreateRequest): LeaseCreateWireRequest {
  return {
    property_id: data.propertyId,
    tenant_contact_id: data.tenantContactId,
    start_date: data.startDate,
    end_date: data.endDate,
    rent_amount_kopecks: data.rentKopecks,
    deposit_amount_kopecks: data.depositKopecks,
    payment_day: data.paymentDay,
    comment: data.comment,
  };
}

export function toUpdateWireRequest(data: LeaseUpdateRequest): LeaseUpdateWireRequest {
  return {
    tenant_contact_id: data.tenantContactId,
    clear_tenant_contact: data.clearTenantContact,
    start_date: data.startDate,
    end_date: data.endDate,
    rent_amount_kopecks: data.rentKopecks,
    deposit_amount_kopecks: data.depositKopecks,
    payment_day: data.paymentDay,
    comment: data.comment,
  };
}

function fetchLease(id: string): Promise<Lease> {
  return apiClient<LeaseResponse>(`/leases/${id}`).then(mapLeaseResponse);
}

function invalidateLeaseScope(
  queryClient: ReturnType<typeof useQueryClient>,
  lease: Pick<Lease, 'id' | 'propertyId'>,
): void {
  void queryClient.invalidateQueries({ queryKey: leaseKeys.all });
  void queryClient.invalidateQueries({ queryKey: leaseKeys.detail(lease.id) });
  void queryClient.invalidateQueries({
    queryKey: operationKeys.operations({ lease_id: lease.id }),
  });
  void queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
  void queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
  void queryClient.invalidateQueries({ queryKey: financeKeys.reports() });
  if (lease.propertyId) {
    void queryClient.invalidateQueries({ queryKey: propertyKeys.list });
    void queryClient.invalidateQueries({ queryKey: propertyKeys.detail(lease.propertyId) });
    void queryClient.invalidateQueries({
      queryKey: leaseKeys.byProperty(lease.propertyId),
    });
    void queryClient.invalidateQueries({
      queryKey: operationKeys.byProperty(lease.propertyId),
    });
    void queryClient.invalidateQueries({
      queryKey: operationKeys.summary(lease.propertyId),
    });
  }
}

export function useLeases(): UseQueryResult<Lease[], ApiError> {
  return useQuery({
    queryKey: leaseKeys.all,
    queryFn: async () => {
      const response = await apiClient<LeasesResponse>('/leases');
      return response.items.map(mapLeaseResponse);
    },
  });
}

export function useLease(id: string): UseQueryResult<Lease, ApiError> {
  return useQuery({
    queryKey: leaseKeys.detail(id),
    queryFn: () => fetchLease(id),
    enabled: Boolean(id),
  });
}

export function useCreateLease(): UseMutationResult<
  Lease,
  ApiError,
  LeaseCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data) =>
      apiClient<LeaseResponse>('/leases', {
        method: 'POST',
        body: JSON.stringify(toCreateWireRequest(data)),
      }).then(mapLeaseResponse),
    onSuccess: (lease) => {
      invalidateLeaseScope(queryClient, lease);
    },
  });
}

export function useUpdateLease(): UseMutationResult<
  Lease,
  ApiError,
  { id: string; data: LeaseUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<LeaseResponse>(`/leases/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(toUpdateWireRequest(data)),
      }).then(mapLeaseResponse),
    onSuccess: (lease) => {
      invalidateLeaseScope(queryClient, lease);
    },
  });
}

export function useCompleteLease(): UseMutationResult<
  Lease,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) =>
      apiClient<LeaseResponse>(`/leases/${id}/complete`, {
        method: 'POST',
      }).then(mapLeaseResponse),
    onSuccess: (lease) => {
      invalidateLeaseScope(queryClient, lease);
    },
  });
}

export function useLeaseReminders(
  id: string,
): UseQueryResult<RemindersResponse, ApiError> {
  return useQuery({
    queryKey: leaseKeys.reminders(id),
    queryFn: () => apiClient<RemindersResponse>(`/leases/${id}/reminders`),
    enabled: Boolean(id),
  });
}

export function useCreateLeaseReminder(): UseMutationResult<
  ReminderResponse,
  ApiError,
  { id: string; data: ReminderCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<ReminderResponse>(`/leases/${id}/reminders`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      void queryClient.invalidateQueries({ queryKey: leaseKeys.reminders(id) });
    },
  });
}
