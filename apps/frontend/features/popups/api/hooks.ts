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
import type { components } from '@/shared/api/dto';

type PendingPopupsResponse = components['schemas']['PendingPopupsResponse'];

// Keys returned by GET /popups/pending. The backend may add new keys over
// time, so the type stays open while the known key gets autocomplete.
export const REMINDERS_ONBOARDING_POPUP_KEY = 'reminders_onboarding';
export type PopupKey = typeof REMINDERS_ONBOARDING_POPUP_KEY | (string & {});

export const popupKeys = {
  pending: ['popups', 'pending'] as const,
};

export function usePendingPopups(): UseQueryResult<PopupKey[], ApiError> {
  return useQuery({
    queryKey: popupKeys.pending,
    queryFn: async () => {
      const res = await apiClient<PendingPopupsResponse>('/popups/pending');
      return res.popups;
    },
    // useMarkPopupSeen keeps the cached list in sync between background
    // refetches, so the provider default staleTime (30s) is enough.
  });
}

export function useMarkPopupSeen(): UseMutationResult<void, ApiError, PopupKey> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (popupKey) =>
      apiClient<void>(`/popups/${popupKey}/seen`, { method: 'POST' }),
    onSuccess: (_, popupKey) => {
      queryClient.setQueryData<PopupKey[]>(popupKeys.pending, (previous) =>
        previous?.filter((key) => key !== popupKey),
      );
      queryClient.invalidateQueries({ queryKey: popupKeys.pending });
    },
  });
}
