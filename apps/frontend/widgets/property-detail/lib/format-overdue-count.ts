export function formatOverdueCount(count: number): string {
  const last = count % 10;
  const lastTwo = count % 100;

  let label: string;
  if (last === 1 && lastTwo !== 11) {
    label = 'просроченный платёж';
  } else if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(lastTwo)) {
    label = 'просроченных платежа';
  } else {
    label = 'просроченных платежей';
  }

  return `${count} ${label}`;
}
