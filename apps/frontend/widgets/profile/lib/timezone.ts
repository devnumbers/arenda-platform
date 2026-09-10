export type TimezoneOption = {
  readonly value: string;
  readonly label: string;
};

/** Все 22 IANA-зоны территории РФ, отсортированы по смещению UTC (запад →
 * восток). Пара: канонический IANA-идентификатор (хранится в /me) и
 * подпись «Город (UTC±N)». Справочник переехал из легаси TimezoneSelect
 * (снесён в #593): подпись нужна строке «Часовой пояс» экрана «Аккаунт»,
 * сами опции — пикеру #594. */
export const TIMEZONE_OPTIONS: readonly TimezoneOption[] = [
  { value: 'Europe/Kaliningrad', label: 'Калининград (UTC+2)' },
  { value: 'Europe/Moscow', label: 'Москва (UTC+3)' },
  { value: 'Europe/Samara', label: 'Самара (UTC+4)' },
  { value: 'Europe/Saratov', label: 'Саратов (UTC+4)' },
  { value: 'Asia/Yekaterinburg', label: 'Екатеринбург (UTC+5)' },
  { value: 'Asia/Omsk', label: 'Омск (UTC+6)' },
  { value: 'Asia/Novosibirsk', label: 'Новосибирск (UTC+7)' },
  { value: 'Asia/Barnaul', label: 'Барнаул (UTC+7)' },
  { value: 'Asia/Tomsk', label: 'Томск (UTC+7)' },
  { value: 'Asia/Novokuznetsk', label: 'Новокузнецк (UTC+7)' },
  { value: 'Asia/Krasnoyarsk', label: 'Красноярск (UTC+7)' },
  { value: 'Asia/Irkutsk', label: 'Иркутск (UTC+8)' },
  { value: 'Asia/Chita', label: 'Чита (UTC+9)' },
  { value: 'Asia/Yakutsk', label: 'Якутск (UTC+9)' },
  { value: 'Asia/Khandyga', label: 'Хандыга (UTC+9)' },
  { value: 'Asia/Vladivostok', label: 'Владивосток (UTC+10)' },
  { value: 'Asia/Ust-Nera', label: 'Усть-Нера (UTC+10)' },
  { value: 'Asia/Magadan', label: 'Магадан (UTC+11)' },
  { value: 'Asia/Sakhalin', label: 'Южно-Сахалинск (UTC+11)' },
  { value: 'Asia/Srednekolymsk', label: 'Среднеколымск (UTC+11)' },
  { value: 'Asia/Kamchatka', label: 'Петропавловск-Камчатский (UTC+12)' },
  { value: 'Asia/Anadyr', label: 'Анадырь (UTC+12)' },
];

/** Подпись строки «Часовой пояс»: город и смещение для зоны справочника,
 * иначе сырой IANA-идентификатор (тихое автосохранение может записать
 * любую зону браузера), пустое значение — пустая строка. */
export function formatTimezoneLabel(timezone: string | null | undefined): string {
  if (!timezone) {
    return '';
  }
  return TIMEZONE_OPTIONS.find((option) => option.value === timezone)?.label ?? timezone;
}
