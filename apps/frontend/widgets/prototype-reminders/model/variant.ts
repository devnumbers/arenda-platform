// PROTOTYPE — throwaway, issue #105

export type PrototypeVariant = 'A' | 'B' | 'C';

export const PROTOTYPE_VARIANT_ORDER: readonly PrototypeVariant[] = ['A', 'B', 'C'];

export const PROTOTYPE_VARIANT_NAMES: Record<PrototypeVariant, string> = {
  A: 'Одноэкранная форма',
  B: 'Визард',
  C: 'Форма с живой сводкой',
};

export const PROTOTYPE_ROUTES = {
  create: '/prototype/reminders/create',
  item: '/prototype/reminders/item',
  objectBlock: '/prototype/reminders/object-block',
} as const;

export function parsePrototypeVariant(value: string | string[] | null | undefined): PrototypeVariant {
  const raw = Array.isArray(value) ? value[0] : value;
  if (raw === 'B' || raw === 'C') {
    return raw;
  }
  return 'A';
}

export function prototypeHref(
  path: string,
  variant: PrototypeVariant,
  extra?: Record<string, string>,
): string {
  const params = new URLSearchParams({ variant, ...extra });
  return `${path}?${params.toString()}`;
}
