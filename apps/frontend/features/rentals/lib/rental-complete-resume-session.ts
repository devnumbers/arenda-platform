'use client';

/**
 * Метка продолжения смены статуса после завершения аренды (гард #628,
 * решение владельца 14.10.2026): подтверждение гарда у идущей аренды
 * уводит в мастер итогов (#534), и выбранный под гардом статус (ремонт
 * или архив) страница объекта применяет сама — после успешного
 * завершения. Носитель — модуль-синглтон, как сессия визарда создания
 * (rental-wizard-session): живёт в памяти SPA и умирает с вкладкой;
 * перезагрузка честно теряет продолжение — статус повторяется вручную,
 * гард после завершения уже не стоит.
 *
 * Контракт трёх ручек: гард страницы объекта ставит метку до ухода в
 * мастер (`request`), мастер после успешной мутации завершения помечает
 * её исполненной (`fulfill`), страница объекта при монтировании потребляет
 * её одноразово (`consume`). Потребление всегда гасит метку: незавершённая
 * (мастер закрыли) статус не даёт — иначе возврат «назад» из мастера
 * применял бы то, чего не было; исполненная ждёт страницу объекта и
 * переживает крюк «Посмотреть аренду» на финале мастера.
 */

export type RentalCompleteResumeAction = 'maintenance' | 'archive';

type RentalCompleteResumeEntry = {
  action: RentalCompleteResumeAction;
  fulfilled: boolean;
};

/** Метка per объект; живёт, пока жива страница. */
const resumes = new Map<string, RentalCompleteResumeEntry>();

/** Гард смены статуса (#628) уходит в мастер — ставит метку с выбранным
 * под гардом статусом. Повторный запрос перезаписывает действие. */
export function requestRentalCompleteResume(
  propertyId: string,
  action: RentalCompleteResumeAction,
): void {
  resumes.set(propertyId, {action, fulfilled: false});
}

/** Мастер завершил аренду успешно — метка исполнена, статус можно
 * применять. Без метки — no-op (мастер открыт не из-под гарда). */
export function fulfillRentalCompleteResume(propertyId: string): void {
  const entry = resumes.get(propertyId);
  if (entry !== undefined) {
    entry.fulfilled = true;
  }
}

/** Страница объекта потребляет метку одноразово: действие — только если
 * завершение в мастере произошло; любое чтение гасит метку. */
export function consumeRentalCompleteResume(
  propertyId: string,
): RentalCompleteResumeAction | null {
  const entry = resumes.get(propertyId);
  if (entry === undefined) {
    return null;
  }
  resumes.delete(propertyId);
  return entry.fulfilled ? entry.action : null;
}
