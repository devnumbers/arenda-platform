/** Адрес бэка для серверных прокси Next (catch-all /api/[...path],
 * стриминговые /api/notifications/stream и /api/realtime/stream, proxy.ts):
 * единственный резолв переменной окружения — все прокси-поверхности Next
 * проксируют один и тот же бэк. */
export const BACKEND_URL = process.env.BACKEND_URL ?? 'http://localhost:8080';
