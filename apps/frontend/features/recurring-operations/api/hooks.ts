'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { recurringOperationKeys } from './keys';
import type { components } from '@/shared/api/generated';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationCreateRequest =
  components['schemas']['RecurringOperationCreateRequest'];
type RecurringOperationUpdateRequest =
  components['schemas']['RecurringOperationUpdateRequest'];
type RecurringOperationsResponse =
  components['schemas']['RecurringOperationsResponse'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useRecurringOperationsByProperty(propertyId: string) {
  return useQuery({
    queryKey: recurringOperationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<RecurringOperationsResponse>(
        `/properties/${propertyId}/recurring-operations`,
      ),
    enabled: Boolean(propertyId),
  });
}

export function useRecurringOperation(id: string) {
  return useQuery({
    queryKey: recurringOperationKeys.detail(id),
    queryFn: () =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      data,
    }: {
      propertyId: string;
      data: RecurringOperationCreateRequest;
    }) =>
      apiClient<RecurringOperationResponse>(
        `/properties/${propertyId}/recurring-operations`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
    },
  });
}

export function useUpdateRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: RecurringOperationUpdateRequest;
    }) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function usePauseRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<RecurringOperationResponse>(
        `/recurring-operations/${id}/pause`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useResumeRecurringOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<RecurringOperationResponse>(
        `/recurring-operations/${id}/resume`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useRecurringOperationReminders(
  propertyId: string,
  recurringOperationId: string,
) {
  return useQuery({
    queryKey: recurringOperationKeys.reminders(
      propertyId,
      recurringOperationId,
    ),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/recurring-operations/${recurringOperationId}/reminders`,
      ),
    enabled: Boolean(propertyId) && Boolean(recurringOperationId),
  });
}

export function useCreateRecurringOperationReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      recurringOperationId,
      data,
    }: {
      propertyId: string;
      recurringOperationId: string;
      data: ReminderCreateRequest;
    }) =>
      apiClient<ReminderResponse>(
        `/properties/${propertyId}/recurring-operations/${recurringOperationId}/reminders`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId, recurringOperationId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.reminders(
          propertyId,
          recurringOperationId,
        ),
      });
    },
  });
}
