// PROTOTYPE — throwaway, issue #106

export type PrototypeVariant = 'A' | 'B' | 'C';

export const PROTOTYPE_VARIANT_ORDER: readonly PrototypeVariant[] = ['A', 'B', 'C'];

export const PROTOTYPE_VARIANT_NAMES: Record<PrototypeVariant, string> = {
  A: 'Месяц-сетка',
  B: 'Неделя + повестка',
  C: 'Лента по объектам',
};

export const PROTOTYPE_CALENDAR_ROUTE = '/prototype/calendar';

// Поверхности прототипа #105, на которые ссылается календарь (победивший вариант B).
export const PROTOTYPE_REMINDER_CREATE_HREF = '/prototype/reminders/create?variant=B';
export const PROTOTYPE_REMINDER_ITEM_HREF = '/prototype/reminders/item?variant=B';

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
