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

export function useRecurringOperationsByProperty(
  propertyId: string,
): UseQueryResult<RecurringOperationsResponse, ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<RecurringOperationsResponse>(
        `/properties/${propertyId}/recurring-operations`,
      ),
    enabled: Boolean(propertyId),
  });
}

export function useRecurringOperation(
  id: string,
): UseQueryResult<RecurringOperationResponse, ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.detail(id),
    queryFn: () =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { propertyId: string; data: RecurringOperationCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, data }) =>
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

export function useUpdateRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { id: string; propertyId: string; data: RecurringOperationUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function usePauseRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { id: string; propertyId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}/pause`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useResumeRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { id: string; propertyId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<RecurringOperationResponse>(
        `/recurring-operations/${id}/resume`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useRecurringOperationReminders(
  propertyId: string,
  recurringOperationId: string,
): UseQueryResult<RemindersResponse, ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.reminders(propertyId, recurringOperationId),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/recurring-operations/${recurringOperationId}/reminders`,
      ),
    enabled: Boolean(propertyId) && Boolean(recurringOperationId),
  });
}

export function useCreateRecurringOperationReminder(): UseMutationResult<
  ReminderResponse,
  ApiError,
  {
    propertyId: string;
    recurringOperationId: string;
    data: ReminderCreateRequest;
  }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, recurringOperationId, data }) =>
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
