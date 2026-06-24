const DAY_MS = 24 * 60 * 60 * 1000;

export function formatDeadline(iso: string): string {
  const target = new Date(iso).getTime();
  const now = Date.now();
  const diffDays = Math.ceil((target - now) / DAY_MS);

  if (diffDays <= 0) {
    return 'Сегодня';
  }

  return `${diffDays} ${pluralize(diffDays, 'день', 'дня', 'дней')}`;
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
