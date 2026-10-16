/** Тип устройства из User-Agent сессии (ADR 0056, #728). `unknown` —
 * парсер не распознал клиента. */
export type SessionDeviceType = 'computer' | 'phone' | 'tablet' | 'tv' | 'unknown';

/** Активная сессия в списке устройств (identity/CONTEXT.md «Активная
 * сессия», #728): строка экрана «Устройства» (#730). `current` — сессия
 * собственной куки вызывающего: она не завершается ревокацией, только
 * выходом на хабе профиля. */
export type ActiveSession = {
  readonly id: string;
  readonly deviceType: SessionDeviceType;
  /** Браузер с мажорной версией при разборе: «Chrome 121»; пусто — не
   * распознан. */
  readonly browser: string;
  readonly browserMajor: number | null;
  readonly os: string;
  /** GeoIP-город последнего IP в русской локали; null — не определён. */
  readonly city: string | null;
  readonly lastIp: string | null;
  /** RFC3339-момент последней активности (sliding-продление, ADR 0056). */
  readonly lastSeenAt: string;
  readonly createdAt: string;
  readonly current: boolean;
};
