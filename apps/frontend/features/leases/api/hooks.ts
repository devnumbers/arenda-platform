'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { leaseKeys } from './keys';
import { financeKeys } from '@/features/finance/api/keys';
import { operationKeys } from '@/features/operations/api/keys';
import { propertyKeys } from '@/features/properties/api/keys';
import type { components } from '@/shared/api/generated';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeasesResponse = components['schemas']['LeasesResponse'];
type PropertyLeasesResponse = components['schemas']['PropertyLeasesResponse'];
type LeaseCreateRequest = components['schemas']['LeaseCreateRequest'];
type LeaseUpdateRequest = components['schemas']['LeaseUpdateRequest'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useLeases(): UseQueryResult<LeaseResponse[], ApiError> {
  return useQuery({
    queryKey: leaseKeys.all,
    queryFn: async () => {
      const response = await apiClient<LeasesResponse>('/leases');
      return response.items;
    },
  });
}

export function useLease(id: string): UseQueryResult<LeaseResponse, ApiError> {
  return useQuery({
    queryKey: leaseKeys.detail(id),
    queryFn: () => apiClient<LeaseResponse>(`/leases/${id}`),
    enabled: Boolean(id),
  });
}

export function usePropertyLeases(
  propertyId: string,
): UseQueryResult<PropertyLeasesResponse, ApiError> {
  return useQuery({
    queryKey: leaseKeys.byProperty(propertyId),
    queryFn: () => apiClient<PropertyLeasesResponse>(`/properties/${propertyId}/leases`),
    enabled: Boolean(propertyId),
  });
}

export function useCreateLease(): UseMutationResult<
  LeaseResponse,
  ApiError,
  LeaseCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: LeaseCreateRequest) =>
      apiClient<LeaseResponse>('/leases', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (lease) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
      queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
      queryClient.invalidateQueries({ queryKey: financeKeys.reports() });
      if (lease.property_id) {
        queryClient.invalidateQueries({ queryKey: propertyKeys.list });
        queryClient.invalidateQueries({ queryKey: propertyKeys.detail(lease.property_id) });
        queryClient.invalidateQueries({
          queryKey: leaseKeys.byProperty(lease.property_id),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.byProperty(lease.property_id),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.summary(lease.property_id),
        });
      }
    },
  });
}

export function useUpdateLease(): UseMutationResult<
  LeaseResponse,
  ApiError,
  { id: string; data: LeaseUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<LeaseResponse>(`/leases/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (lease, { id }) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(id) });
      queryClient.invalidateQueries({
        queryKey: operationKeys.operations({ lease_id: id }),
      });
      queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
      queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
      queryClient.invalidateQueries({ queryKey: financeKeys.reports() });
      if (lease.property_id) {
        queryClient.invalidateQueries({ queryKey: propertyKeys.list });
        queryClient.invalidateQueries({ queryKey: propertyKeys.detail(lease.property_id) });
        queryClient.invalidateQueries({
          queryKey: leaseKeys.byProperty(lease.property_id),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.byProperty(lease.property_id),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.summary(lease.property_id),
        });
      }
    },
  });
}

export function useCompleteLease(): UseMutationResult<
  LeaseResponse,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) =>
      apiClient<LeaseResponse>(`/leases/${id}/complete`, {
        method: 'POST',
      }),
    onSuccess: (lease) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(lease.id) });
      queryClient.invalidateQueries({
        queryKey: operationKeys.operations({ lease_id: lease.id }),
      });
      queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
      queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
      queryClient.invalidateQueries({ queryKey: financeKeys.reports() });
      if (lease.property_id) {
        queryClient.invalidateQueries({ queryKey: propertyKeys.list });
        queryClient.invalidateQueries({ queryKey: propertyKeys.detail(lease.property_id) });
        queryClient.invalidateQueries({
          queryKey: leaseKeys.byProperty(lease.property_id),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.byProperty(lease.property_id),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.summary(lease.property_id),
        });
      }
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
      queryClient.invalidateQueries({ queryKey: leaseKeys.reminders(id) });
    },
  });
}
