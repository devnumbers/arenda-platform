'use client';

import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapRental } from '@/entities/rental';
import type { Rental, RentalCreateCommand, RentalUpdateCommand } from '@/entities/rental';
import { paymentKeys, paymentOperationKeys, rentalKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type RentalResponseDto = components['schemas']['RentalResponse'];
type RentalsResponseDto = components['schemas']['RentalsResponse'];

/**
 * Создание аренды (#530): POST /properties/{propertyId}/rentals — аренда и
 * её Платёж арендной платы создаются атомарно (ADR 0053 §3). Вторая
 * незавершённая аренда на объекте — 409 с апп-ошибкой. Инвалидация:
 * платежи объекта (появляется платёж аренды) и весь срез rentals.
 */
export function useCreateRental(
  propertyId: string,
): UseMutationResult<Rental, ApiError, RentalCreateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: RentalCreateCommand) => {
      const response = await apiClient<RentalResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/rentals`,
        { method: 'POST', body: JSON.stringify(command) },
      );
      return mapRental(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: rentalKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
    },
  });
}

/**
 * Список аренд объекта (#531): плоский список без пагинации — незавершённая
 * (ожидающая/активная) первой, далее завершённые по дате завершения
 * (ADR 0053 §4). Экран «Аренда» живёт первой (незавершённой) арендой;
 * завершённые — материал «Прошлых аренд» (#535).
 */
export function useRentals(propertyId: string): UseQueryResult<Rental[], ApiError> {
  return useQuery({
    queryKey: rentalKeys.list(propertyId),
    queryFn: async () => {
      const response = await apiClient<RentalsResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/rentals`,
      );
      return response.items.map(mapRental);
    },
    enabled: Boolean(propertyId),
  });
}

/**
 * Правка условий аренды (#532): частичный PATCH — команда уже посчитана
 * диффом формы (edit-model), опущенное поле остаётся без изменений, явный
 * null очищает (tri-state ADR 0053 §4). Сервер в транзакции синхронно правит
 * Платёж арендной платы (сумма, день оплаты, автоплатёж, окончание) и гоняет
 * тик — инвалидируются аренды, правила и операции. Начало не правится;
 * завершённая аренда — 409.
 */
export function useUpdateRental(
  propertyId: string,
  rentalId: string,
): UseMutationResult<Rental, ApiError, RentalUpdateCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: RentalUpdateCommand) => {
      const response = await apiClient<RentalResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/rentals/${encodeURIComponent(rentalId)}`,
        { method: 'PATCH', body: JSON.stringify(command) },
      );
      return mapRental(response);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: rentalKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}
