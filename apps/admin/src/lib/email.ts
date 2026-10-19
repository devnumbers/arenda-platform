/**
 * Проверка почты по форме «что-то@домен.тлд» — та же свободная форма, что
 * и на фронте кабинета (apps/frontend/shared/lib/email.ts) и в доменах
 * identity и contacts на бэкенде: каноническую форму адреса (обрезка +
 * нижний регистр) придаёт бэкенд, клиентская проверка — защита от опечаток.
 */
export function isEmailValid(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}
