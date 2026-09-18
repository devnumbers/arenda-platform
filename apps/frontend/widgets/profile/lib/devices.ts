import type { SessionDevice, SessionDeviceType } from '@/entities/session';
import { formatSessionLastSeen } from '@/shared/lib/date-format';

/** Отображение активных сессий на экране «Устройства» (#730, мок
 * 1804-105061): текущая сессия — секция «Это устройство» с синим «В
 * сети», прочие — «Активные сессии» с моментом последней активности.
 * Сортировку не трогает — бэк отдаёт по свежей активности (#728). */

/** Текущая сессия отделяется от прочих; порядок бэка сохраняется. */
export function splitSessions(
  sessions: ReadonlyArray<SessionDevice>,
): { current: SessionDevice | null; others: SessionDevice[] } {
  return {
    current: sessions.find((session) => session.current) ?? null,
    others: sessions.filter((session) => !session.current),
  };
}

/** Заголовок строки: «Chrome 121» (моки 1804-105061/1903-39135);
 * нераспознанный браузер — ОС; не распознано ничего — «Устройство». */
export function sessionTitle(session: SessionDevice): string {
  const browser = [session.browser, session.browserMajor]
    .filter((part) => part !== '' && part !== null)
    .join(' ');
  if (browser !== '') return browser;
  return session.os !== '' ? session.os : 'Устройство';
}

/** Иконка типа устройства из канона (§10): в макете два глифа —
 * Icon/R/Phone и Icon/R/Computer (1804:105303 / 1804:105311). Планшет и
 * ТВ рисуются монитором (большой экран), нераспознанное — компьютером;
 * отдельного глифа в каноне нет. */
export function deviceIconName(deviceType: SessionDeviceType): 'phone' | 'computer' {
  return deviceType === 'phone' ? 'phone' : 'computer';
}

/** Подзаголовок строки и его тон: текущая — «В сети • Город» (синий
 * #2B7FFF мока), прочие — момент активности • Город (серый #6F787C). */
export function sessionSubtitle(
  session: SessionDevice,
  now: Date,
): { text: string; online: boolean } {
  const activity = session.current ? 'В сети' : formatSessionLastSeen(session.lastSeenAt, now);
  const text = session.city === null ? activity : `${activity} • ${session.city}`;
  return { text, online: session.current };
}
