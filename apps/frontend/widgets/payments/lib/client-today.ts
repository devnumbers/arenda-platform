import type { IsoDate } from '@/entities/payment';

/**
 * «Сегодня» клиентской проекции подзаголовков строк (дата следующего
 * вхождения): локальная дата браузера как date-строка 'YYYY-MM-DD'.
 * Отступление осознанное (спека #453: «сегодня» интерфейса — по TZ
 * собственника, сервер отдаёт рассчитанные статусы): у контракта списка
 * платежей поля следующей даты нет, проекция — клиентский порт прототипа.
 * Расхождение с TZ собственника ограничено краевыми часами суток.
 */
export function clientTodayIso(now: Date = new Date()): IsoDate {
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}
