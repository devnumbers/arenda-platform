const rubFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

export function formatMoneyKopecks(kopecks: number): string {
  return rubFormatter.format(kopecks / 100);
}
