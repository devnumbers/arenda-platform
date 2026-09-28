import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapRental } from '@/entities/rental';
import type { Rental } from '@/entities/rental';
import type { components } from '@/shared/api/dto';
import { rentalKeys } from '@/shared/api/query-keys';

type RentalsResponseDto = components['schemas']['RentalsResponse'];

/** Двойной модуль API-слоя rentals (без 'use client'): чистые fetch-функции
 * и queryOptions-фабрики канона #887 — общий источник ключ+fetch для
 * клиентских хуков и серверного префетча. Хуки — в hooks.ts ('use client'). */

/** Чистый fetch списка аренд объекта — общее горло хука и серверного
 * префетча #887. */
export async function fetchRentals(
  propertyId: string,
  transport: ApiTransport = apiClient,
): Promise<Rental[]> {
  const response = await transport<RentalsResponseDto>(
    `/properties/${encodeURIComponent(propertyId)}/rentals`,
  );
  return response.items.map(mapRental);
}

/** Опции списка аренд объекта (канон #887): один источник ключ+fetch для
 * хука и серверного префетча. */
export function rentalsQueryOptions({
  propertyId,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<Rental[], ApiError, Rental[], ReturnType<typeof rentalKeys.list>> {
  return queryOptions({
    queryKey: rentalKeys.list(propertyId),
    queryFn: () => fetchRentals(propertyId, transport),
  });
}

