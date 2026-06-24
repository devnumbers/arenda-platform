export function formatLeaseMonth(start: string): string {
  const months = Math.max(
    1,
    Math.floor(
      (Date.now() - new Date(start).getTime()) / (1000 * 60 * 60 * 24 * 30),
    ),
  );
  const last = months % 10;
  const label =
    last === 1 && months !== 11
      ? 'месяц'
      : [2, 3, 4].includes(last) && ![12, 13, 14].includes(months % 100)
        ? 'месяца'
        : 'месяцев';
  return `${months} ${label}`;
}
