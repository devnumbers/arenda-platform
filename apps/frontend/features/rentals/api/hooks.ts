'use client';

import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { mapRental, mapRentalSummary } from '@/entities/rental';
import type {
  Rental,
  RentalCompleteCommand,
  RentalCreateCommand,
  RentalSummary,
  RentalUpdateCommand,
} from '@/entities/rental';
import type { IsoDate } from '@/shared/lib/calendar';
import { paymentKeys, paymentOperationKeys, rentalKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type RentalResponseDto = components['schemas']['RentalResponse'];
type RentalSummaryDto = components['schemas']['RentalSummaryResponse'];
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

/**
 * Завершение аренды (#534): POST …/complete с фактической датой и записью
 * возврата залога (ADR 0053 §3). Сервер останавливает Платёж на дате
 * завершения — будущие плановые операции сносятся: инвалидируются аренды,
 * платежи и операции. Повторное завершение — 409.
 */
export function useCompleteRental(
  propertyId: string,
  rentalId: string,
): UseMutationResult<Rental, ApiError, RentalCompleteCommand> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (command: RentalCompleteCommand) => {
      const response = await apiClient<RentalResponseDto>(
        `/properties/${encodeURIComponent(propertyId)}/rentals/${encodeURIComponent(rentalId)}/complete`,
        { method: 'POST', body: JSON.stringify(command) },
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

/**
 * Итоги аренды (#534, решение №13): все paid-операции объекта за
 * [начало, until] по дате вхождения — как период на странице операций,
 * суженный объектом. Мастер завершения превьюит с выбранной датой до
 * фактического завершения; `until` в ключе — расчёт меняется с датой.
 */
export function useRentalSummary(
  propertyId: string,
  rentalId: string,
  until: IsoDate,
): UseQueryResult<RentalSummary, ApiError> {
  return useQuery({
    queryKey: rentalKeys.summary(propertyId, rentalId, until),
    queryFn: async () => {
      const response = await apiClient<RentalSummaryDto>(
        `/properties/${encodeURIComponent(propertyId)}/rentals/${encodeURIComponent(rentalId)}/summary?until=${until}`,
      );
      return mapRentalSummary(response);
    },
    enabled: Boolean(propertyId && rentalId && until),
  });
}

/**
 * Удаление аренды (#535): DELETE …/rentals/{rentalId} — разрешён только
 * не начавшейся или завершённой (начавшаяся незавершённая — 409, ADR 0053).
 * Сервер в транзакции удаляет управляемый Платёж: будущие плановые
 * вхождения сносятся, просроченные остаются долгом, оплаченные — факты
 * истории с платежом «удалён» (семантика payments, тикет #446).
 * Кэш операций удалённого платежа снимается синхронно (removeQueries —
 * конвенция удалений), срезы платежей и операций объекта инвалидируются —
 * оплаченные переживают удаление на экранах операций.
 */
export function useDeleteRental(
  propertyId: string,
  rentalId: string,
  paymentId: string,
): UseMutationResult<void, ApiError, void> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await apiClient<void>(
        `/properties/${encodeURIComponent(propertyId)}/rentals/${encodeURIComponent(rentalId)}`,
        { method: 'DELETE' },
      );
    },
    onSuccess: () => {
      queryClient.removeQueries({
        queryKey: paymentOperationKeys.byPaymentPrefix(propertyId, paymentId),
      });
      queryClient.removeQueries({
        queryKey: paymentOperationKeys.byPaymentPagedPrefix(propertyId, paymentId),
      });
      void queryClient.invalidateQueries({ queryKey: rentalKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentKeys.all });
      void queryClient.invalidateQueries({ queryKey: paymentOperationKeys.all });
    },
  });
}
