import { ApiError } from '@/shared/api/errors';

export type PropertyParticipantsErrorKind = 'not_found' | 'generic';

// Разводит ошибку списка участников объекта (#719): 404 — объекта нет или
// свой доступ отозван/приостановлен (бэк скрывает нечитаемый объект как
// «не найден», policy.CanView), экран показывает not-found-состояние; всё
// остальное — generic-ошибка с кнопкой «Повторить».
export function resolvePropertyParticipantsError(
  error: unknown,
): PropertyParticipantsErrorKind {
  if (error instanceof ApiError && error.status === 404) {
    return 'not_found';
  }
  return 'generic';
}
