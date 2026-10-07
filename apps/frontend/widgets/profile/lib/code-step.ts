import type { ApiError } from '@/shared/api/errors';
import { isValidLoginCode } from '@/shared/lib/login-code';

/** Кулдаун повторной отправки кода — серверный троттлинг identity 1 мин
 * (тикет #733, макет 1869-68137: подпись «через 00:59»).
 * Граница отображения: подпись стартует с «00:59» (последняя секунда
 * дедлайна не рисуется), поэтому плитка разблокируется на ≤1 с раньше
 * серверного троттлинга; ранний клик получает честный 429-тост сценария. */
export const RESEND_COOLDOWN_MS = 60_000;

/** Куда идёт ошибка verify-мутации шага кода (change-phone / change-email):
 * неверный код (401) — inline в error-проп поля кода (макет 2343-51004,
 * текст — detail бэка «Неверный код»); остальные API-ошибки (429, 409,
 * блокировка) — null, их ведут тосты сценариев профиля. */
export function invalidCodeDetail(error: ApiError): string | null {
  return error.status === 401 ? error.detail : null;
}

/** Ошибка error-проп поля кода: после сабмита неполный код — маска-подсказка
 * «Введите 6-значный код» (сильнее inline), иначе — inline 401 verify-мутации,
 * если он выставлен. */
export function codeFieldError(
  isSubmitAttempted: boolean,
  value: string,
  inlineError: string | null,
): string | undefined {
  if (isSubmitAttempted && !isValidLoginCode(value)) {
    return 'Введите 6-значный код';
  }
  return inlineError ?? undefined;
}
