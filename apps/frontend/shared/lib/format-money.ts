export function formatMoneyKopecks(
  kopecks: number,
  options: { round?: boolean } = {},
): string {
  if (!Number.isFinite(kopecks)) return '';
  const value = options.round ? Math.round(kopecks / 100) : kopecks / 100;
  return `${value.toLocaleString('ru-RU')} ₽`;
}
