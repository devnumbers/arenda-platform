export function formatMoneyKopecks(kopecks: number): string {
  if (!Number.isFinite(kopecks)) return '';
  return `${(kopecks / 100).toLocaleString('ru-RU')} ₽`;
}
