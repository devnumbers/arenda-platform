import { ApiError } from './errors';

/**
 * 404 как «ресурса нет» (#768): детерминированный 404 эндпоинта, для
 * которого отсутствие — ожидаемое состояние (подписка сид-юзера), маппится
 * в null вместо вечного pending или error-состояния у потребителя.
 * Остальные ошибки (сеть, 4xx/5xx) летят как есть. 404 в консоли браузера
 * остаётся — это шум транспорта, не состояния приложения.
 */
export async function nullOn404<T>(request: () => Promise<T>): Promise<T | null> {
  try {
    return await request();
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return null;
    }
    throw error;
  }
}
