export function formatMoneyKopecks(kopecks: number): string {
  return `${(kopecks / 100).toLocaleString('ru-RU')} ₽`;
}
