import { ApiError } from '@/shared/api/errors';

export type PropertyDetailErrorKind = 'not_found' | 'suspended' | 'generic';

// Разводит ошибку основного запроса объекта по экранам:
// 404 — нет объекта или нет доступа (чужой/отозванный доступ скрыт как «не найден»),
// 403 + membership_suspended — доступ приостановлен из-за лимита тарифа,
// всё остальное — generic-экран с кнопкой «Повторить».
export function resolvePropertyDetailError(error: unknown): PropertyDetailErrorKind {
  if (error instanceof ApiError) {
    if (error.status === 404) return 'not_found';
    if (error.status === 403 && error.code === 'membership_suspended') return 'suspended';
  }
  return 'generic';
}
