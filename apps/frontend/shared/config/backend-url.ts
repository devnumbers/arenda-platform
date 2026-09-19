/** Адрес бэка для серверных прокси Next (catch-all /api/[...path],
 * стриминговый /api/notifications/stream, proxy.ts): единственный резолв
 * переменной окружения — три поверхности проксируют один и тот же бэк. */
export const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
