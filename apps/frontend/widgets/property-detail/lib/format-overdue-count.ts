export function formatOverdueCount(count: number): string {
  const last = count % 10;
  const lastTwo = count % 100;

  let label: string;
  if (last === 1 && lastTwo !== 11) {
    label = 'просроченная операция';
  } else if ([2, 3, 4].includes(last) && ![12, 13, 14].includes(lastTwo)) {
    label = 'просроченные операции';
  } else {
    label = 'просроченных операций';
  }

  return `${count} ${label}`;
}
