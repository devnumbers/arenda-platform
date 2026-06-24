const DAY_MS = 24 * 60 * 60 * 1000;

export function formatRemainingDuration(start: string, end?: string | null): string {
  if (!end) {
    return 'Бессрочно';
  }

  const startDate = new Date(start);
  const endDate = new Date(end);
  const now = Date.now();

  if (endDate.getTime() <= now) {
    return 'Срок истёк';
  }

  const totalDays = Math.max(1, Math.ceil((endDate.getTime() - Math.max(startDate.getTime(), now)) / DAY_MS));

  if (totalDays < 7) {
    return `${totalDays} ${pluralize(totalDays, 'день', 'дня', 'дней')}`;
  }

  const weeks = Math.floor(totalDays / 7);
  const days = totalDays % 7;
  const weeksText = `${weeks} ${pluralize(weeks, 'неделя', 'недели', 'недель')}`;

  if (days === 0) {
    return weeksText;
  }

  return `${weeksText}, ${days} ${pluralize(days, 'день', 'дня', 'дней')}`;
}

export function formatCurrentLeaseMonth(start: string): string {
  const startDate = new Date(start);
  const now = new Date();

  const months =
    (now.getFullYear() - startDate.getFullYear()) * 12 +
    (now.getMonth() - startDate.getMonth()) +
    1;

  return `${months} месяц`;
}

function pluralize(count: number, one: string, few: string, many: string): string {
  const mod10 = count % 10;
  const mod100 = count % 100;

  if (mod10 === 1 && mod100 !== 11) {
    return one;
  }

  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) {
    return few;
  }

  return many;
}
