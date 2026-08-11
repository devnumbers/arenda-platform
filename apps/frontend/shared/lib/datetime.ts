import { formatCountdownLabel } from '@/shared/lib/format-countdown';

function padTwo(value: number): string {
  return String(value).padStart(2, '0');
}

/**
 * Собирает локальную дату (`YYYY-MM-DD`) и время (`HH:MM`) в RFC 3339 date-time
 * со смещением браузера. Бэкенд свободных напоминаний интерпретирует wall-clock
 * компоненты (год/месяц/день/час/минута) после перевода входного инстанта в
 * часовой пояс владельца, поэтому смещение должно совпадать с поясом владельца.
 * Смещение браузера равно сохранённому IANA-поясу пользователя при автоопределении
 * и дефолте Europe/Moscow — поэтому RFC 3339 со смещением браузера корректен.
 */
export function buildTriggerAt(date: string, time: string): string {
  const instant = new Date(`${date}T${time}:00`);
  const offsetMinutes = -instant.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? '+' : '-';
  const absoluteMinutes = Math.abs(offsetMinutes);
  const offsetHours = padTwo(Math.floor(absoluteMinutes / 60));
  const offsetMins = padTwo(absoluteMinutes % 60);

  return (
    `${instant.getFullYear()}-${padTwo(instant.getMonth() + 1)}-${padTwo(instant.getDate())}` +
    `T${padTwo(instant.getHours())}:${padTwo(instant.getMinutes())}:00` +
    `${sign}${offsetHours}:${offsetMins}`
  );
}

/**
 * Разбивает RFC 3339 date-time (UTC) на локальные дату и время для отображения.
 * Возвращает дату в формате «12 марта» и время в формате «10:00».
 */
export function formatReminderDateTime(triggerAt: string): { date: string; time: string } {
  const instant = new Date(triggerAt);
  if (Number.isNaN(instant.getTime())) {
    return { date: triggerAt, time: '' };
  }

  const date = instant.toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
  });
  const time = instant.toLocaleTimeString('ru-RU', {
    hour: '2-digit',
    minute: '2-digit',
  });

  return { date, time };
}

/**
 * Человекочитаемый отсчёт до срабатывания напоминания от текущего момента.
 * Использует существующий formatCountdownLabel: «Сегодня», «Завтра»,
 * «через N дней», «через N неделю» и т.д. Первая буква капитализируется
 * для отображения в hero-блоке страницы просмотра.
 */
export function formatCountdownFromNow(triggerAt: string): string {
  const instant = new Date(triggerAt);
  if (Number.isNaN(instant.getTime())) {
    return '';
  }

  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfTarget = new Date(instant.getFullYear(), instant.getMonth(), instant.getDate());
  const days = Math.round(
    (startOfTarget.getTime() - startOfToday.getTime()) / (1000 * 60 * 60 * 24),
  );

  const label = formatCountdownLabel(days);
  return label.charAt(0).toUpperCase() + label.slice(1);
}

/**
 * Извлекает локальную дату (`YYYY-MM-DD`) из RFC 3339 date-time.
 * Используется для предзаполнения DateSelect при редактировании.
 */
export function extractLocalDate(triggerAt: string): string {
  const instant = new Date(triggerAt);
  if (Number.isNaN(instant.getTime())) {
    return '';
  }

  return `${instant.getFullYear()}-${padTwo(instant.getMonth() + 1)}-${padTwo(instant.getDate())}`;
}

/**
 * Извлекает локальное время (`HH:MM`) из RFC 3339 date-time.
 * Используется для предзаполнения `<input type="time">` при редактировании.
 */
export function extractLocalTime(triggerAt: string): string {
  const instant = new Date(triggerAt);
  if (Number.isNaN(instant.getTime())) {
    return '';
  }

  return `${padTwo(instant.getHours())}:${padTwo(instant.getMinutes())}`;
}
