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
import type { components } from '@/shared/api/generated';
import { categoryKeys, type OperationCategoryType } from './keys';

type OperationCategory = components['schemas']['OperationCategory'];
type OperationCategoryCreateRequest =
  components['schemas']['OperationCategoryCreateRequest'];

export function useOperationCategories(
  type: OperationCategoryType,
): UseQueryResult<OperationCategory[], ApiError> {
  return useQuery({
    queryKey: categoryKeys.list(type),
    queryFn: () =>
      apiClient<OperationCategory[]>(`/operation-categories?type=${type}`),
  });
}

export function useCreateOperationCategory(): UseMutationResult<
  OperationCategory,
  ApiError,
  OperationCategoryCreateRequest
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data) =>
      apiClient<OperationCategory>('/operation-categories', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (category) => {
      // Seed the cache so the new category shows up instantly, then revalidate.
      queryClient.setQueryData<OperationCategory[]>(
        categoryKeys.list(category.type),
        (previous) => (previous ? [...previous, category] : [category]),
      );
      queryClient.invalidateQueries({
        queryKey: categoryKeys.list(category.type),
      });
    },
  });
}
