/**
 * Ввод суммы numpad'ом (тикет #459, Figma 834:19662): значение — строка
 * рублей с запятой («1234,5»), как её набирает пользователь. Пересчёт в
 * копейки — только через canonical-модуль денег (parseRublesToKopecks);
 * экранная сумма — formatMoneyKopecks(numpadKopecks(value)). Сырой
 * арифметики с деньгами здесь нет.
 */
import { parseRublesToKopecks } from '@/shared/lib/format-money';

export type NumpadDigitKey = '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9';
export type NumpadKey = NumpadDigitKey | ',' | 'erase';

/** Раскладка numpad 3×4 (Figma 834:19678): 1..9, «,», 0, ластик. */
export const NUMPAD_KEYS: ReadonlyArray<NumpadKey> = [
  '1',
  '2',
  '3',
  '4',
  '5',
  '6',
  '7',
  '8',
  '9',
  ',',
  '0',
  'erase',
];

/** Ограничения ввода: 2 копеечных разряда (точность копеек) и 7 разрядов
 * рублей (до 9 999 999,99 ₽ — запас сверх любого реального платежа,
 * держит значение далеко от предела точности double). */
const MAX_RUBLE_DIGITS = 7;
const MAX_KOPECK_DIGITS = 2;

/** Один нажатый клавиш поверх текущего значения; некорректные нажатия
 * (вторая запятая, третий копеечный разряд, лишний разряд рублей)
 * игнорируются — возврат того же значения. */
export function inputNumpadKey(value: string, key: NumpadKey): string {
  if (key === 'erase') {
    return value.slice(0, -1);
  }
  if (key === ',') {
    if (value.includes(',')) return value;
    return value === '' ? '0,' : `${value},`;
  }
  const [rubles = '', kopecks] = value.split(',');
  if (kopecks !== undefined) {
    if (kopecks.length >= MAX_KOPECK_DIGITS) return value;
  } else {
    const significant = rubles.startsWith('0') ? rubles.slice(1) : rubles;
    if (significant.length >= MAX_RUBLE_DIGITS) return value;
  }
  if (value === '' || value === '0') return key;
  return value + key;
}

/** Значение в целых копейках; пустая строка и нули — это 0. */
export function numpadKopecks(value: string): number {
  return parseRublesToKopecks(value) ?? 0;
}
