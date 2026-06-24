export function formatMoney(kopecks: number): string {
  const rubles = Math.round(kopecks / 100);
  return `${rubles.toLocaleString('ru-RU')} ₽`;
}
