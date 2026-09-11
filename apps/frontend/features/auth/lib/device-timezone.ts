/** Часовой пояс устройства для автодетекта при регистрации (#451): логин-экран
 * отправляет его с каждой верификацией кода, бэк применяет зону только при
 * создании аккаунта. undefined — когда браузер не отдал зону, тогда поле в
 * запрос не попадает и аккаунт рождается на дефолте Europe/Moscow. */
export function deviceTimezone(): string | undefined {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return timezone ? timezone : undefined;
}
