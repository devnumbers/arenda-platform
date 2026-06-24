export function formatDuration(start?: string, end?: string): string {
  if (!start || !end) return '';
  const diff = new Date(end).getTime() - new Date(start).getTime();
  const days = Math.max(0, Math.floor(diff / (1000 * 60 * 60 * 24)));
  const weeks = Math.floor(days / 7);
  const remDays = days % 7;
  if (weeks === 0) return `${remDays} ${declDays(remDays)}`;
  return `${weeks} ${declWeeks(weeks)}${remDays ? `, ${remDays} ${declDays(remDays)}` : ''}`;
}

function declDays(n: number): string {
  const last = n % 10;
  if (last === 1 && n !== 11) return 'день';
  if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(n % 100)) return 'дня';
  return 'дней';
}

function declWeeks(n: number): string {
  const last = n % 10;
  if (last === 1 && n !== 11) return 'неделя';
  if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(n % 100)) return 'недели';
  return 'недель';
}
