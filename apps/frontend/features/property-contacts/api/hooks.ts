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
import { mapPropertyContactResponse } from '@/entities/property-contact';
import type { PropertyContact } from '@/entities/property-contact';
import { propertyContactKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type PropertyContactResponse = components['schemas']['PropertyContactResponse'];
type PropertyContactCreateRequest =
  components['schemas']['PropertyContactCreateRequest'];
type PropertyContactUpdateRequest =
  components['schemas']['PropertyContactUpdateRequest'];
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

export function usePropertyContact(
  propertyId: string,
  contactId: string,
): UseQueryResult<PropertyContact, ApiError> {
  return useQuery({
    queryKey: propertyContactKeys.detail(propertyId, contactId),
    queryFn: async () => {
      const response = await apiClient<PropertyContactResponse>(
        `/properties/${propertyId}/contacts/${contactId}`,
      );
      return mapPropertyContactResponse(response);
    },
    enabled: Boolean(propertyId) && Boolean(contactId),
  });
}

export function useUpdatePropertyContact(
  propertyId: string,
  contactId: string,
): UseMutationResult<
  PropertyContact,
  ApiError,
  PropertyContactUpdateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: PropertyContactUpdateRequest) => {
      const response = await apiClient<PropertyContactResponse>(
        `/properties/${propertyId}/contacts/${contactId}`,
        {
          method: 'PATCH',
          body: JSON.stringify(data),
        },
      );
      return mapPropertyContactResponse(response);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: propertyContactKeys.list(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: propertyContactKeys.detail(propertyId, contactId),
      });
    },
  });
}

export function useDeletePropertyContact(
  propertyId: string,
): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (contactId: string) => {
      await apiClient<void>(
        `/properties/${propertyId}/contacts/${contactId}`,
        {
          method: 'DELETE',
        },
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: propertyContactKeys.list(propertyId),
      });
    },
  });
}
