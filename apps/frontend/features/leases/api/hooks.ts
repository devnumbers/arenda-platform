'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { leaseKeys } from './keys';
import type { components } from '@/shared/api/generated';

type LeaseResponse = components['schemas']['LeaseResponse'];
type LeaseCreateRequest = components['schemas']['LeaseCreateRequest'];
type LeaseUpdateRequest = components['schemas']['LeaseUpdateRequest'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useLeases() {
  return useQuery({
    queryKey: leaseKeys.all,
    queryFn: () => apiClient<LeaseResponse[]>('/leases'),
  });
}

export function useLease(id: string) {
  return useQuery({
    queryKey: leaseKeys.detail(id),
    queryFn: () => apiClient<LeaseResponse>(`/leases/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateLease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: LeaseCreateRequest) =>
      apiClient<LeaseResponse>('/leases', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
    },
  });
}

export function useUpdateLease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: LeaseUpdateRequest }) =>
      apiClient<LeaseResponse>(`/leases/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(id) });
    },
  });
}

export function useCompleteLease() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<LeaseResponse>(`/leases/${id}/complete`, {
        method: 'POST',
      }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      queryClient.invalidateQueries({ queryKey: leaseKeys.detail(id) });
    },
  });
}

export function useLeaseReminders(id: string) {
  return useQuery({
    queryKey: leaseKeys.reminders(id),
    queryFn: () => apiClient<RemindersResponse>(`/leases/${id}/reminders`),
    enabled: Boolean(id),
  });
}

export function useCreateLeaseReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: ReminderCreateRequest }) =>
      apiClient<ReminderResponse>(`/leases/${id}/reminders`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: leaseKeys.reminders(id) });
    },
  });
}
