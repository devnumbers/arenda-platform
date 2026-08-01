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
import { mapPropertyContactResponse } from '@/entities/property-contact/model/mappers';
import type { PropertyContact } from '@/entities/property-contact/model/types';
import { propertyContactKeys } from './keys';
import type { components } from '@/shared/api/generated';

type PropertyContactResponse = components['schemas']['PropertyContactResponse'];
type PropertyContactCreateRequest =
  components['schemas']['PropertyContactCreateRequest'];
type PropertyContactsResponse =
  components['schemas']['PropertyContactsResponse'];

export function usePropertyContacts(
  propertyId: string,
): UseQueryResult<PropertyContact[], ApiError> {
  return useQuery({
    queryKey: propertyContactKeys.list(propertyId),
    queryFn: async () => {
      const response = await apiClient<PropertyContactsResponse>(
        `/properties/${propertyId}/contacts`,
      );
      return response.items.map(mapPropertyContactResponse);
    },
    enabled: Boolean(propertyId),
  });
}

export function useCreatePropertyContact(
  propertyId: string,
): UseMutationResult<
  PropertyContact,
  ApiError,
  PropertyContactCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: PropertyContactCreateRequest) => {
      const response = await apiClient<PropertyContactResponse>(
        `/properties/${propertyId}/contacts`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      );
      return mapPropertyContactResponse(response);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: propertyContactKeys.list(propertyId),
      });
    },
  });
}
