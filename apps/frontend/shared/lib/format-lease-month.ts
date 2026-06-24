export function formatLeaseMonth(start: string): string {
  const startDate = new Date(start);
  if (Number.isNaN(startDate.getTime())) return '';
  const now = new Date();
  if (startDate > now) return '';
  const months =
    (now.getFullYear() - startDate.getFullYear()) * 12 +
    (now.getMonth() - startDate.getMonth());
  const normalized = Math.max(0, months);
  const last = normalized % 10;
  const label =
    last === 1 && normalized !== 11
      ? 'месяц'
      : [2, 3, 4].includes(last) && ![12, 13, 14].includes(normalized % 100)
        ? 'месяца'
        : 'месяцев';
  return `${normalized} ${label}`;
}
