/**
 * Ввод суммы с клавиатуры (тикет #459, правка владельца 2026-08-26:
 * numpad убран — обычный текстовый ввод, только цифры). Значение — строка
 * рублей с запятой («1234,5»), как её набирает пользователь; маска
 * sanitizeAmountInput отбрасывает всё лишнее. Пересчёт в копейки — только
 * через canonical-модуль денег (parseRublesToKopecks); экранная сумма —
 * formatMoneyKopecks(amountKopecks(value)). Сырой арифметики с деньгами
 * здесь нет.
 */
import { parseRublesToKopecks } from '@/shared/lib/format-money';

/** Ограничения ввода: 2 копеечных разряда (точность копеек) и 13 разрядов
 * рублей (до ~10 трлн ₽). Потолка сумм нет (ADR 0069, решение владельца
 * 23.10.2026) — это технический предел: при 13 разрядах копейки (до
 * ~9,99×10¹⁴) остаются в точном диапазоне double (Number.MAX_SAFE_INTEGER),
 * арифметика сумм не врёт. */
const MAX_RUBLE_DIGITS = 13;
const MAX_KOPECK_DIGITS = 2;

/** Маска ввода суммы: оставляет только цифры и один десятичный разделитель
 * (точка нормализуется в запятую), схлопывает ведущие нули («00» → «0»,
 * кроме «0,»), ограничивает разряды. Некорректные символы и лишние
 * разряды отбрасываются, а не блокируют ввод. */
export function sanitizeAmountInput(raw: string): string {
  const cleaned = raw.replace(/[^\d.,]/g, '').replace(/\./g, ',');
  const commaIndex = cleaned.indexOf(',');
  const rublesPart = commaIndex === -1 ? cleaned : cleaned.slice(0, commaIndex);
  const kopecksPart = commaIndex === -1 ? '' : cleaned.slice(commaIndex + 1);
  const rubles = rublesPart.replace(/^0+(?=\d)/, '').slice(0, MAX_RUBLE_DIGITS);
  const kopecks = kopecksPart.replace(/,/g, '').slice(0, MAX_KOPECK_DIGITS);
  if (commaIndex === -1) {
    return rubles;
  }
  return `${rubles === '' ? '0' : rubles},${kopecks}`;
}

/** Значение в целых копейках; пустая строка и нули — это 0. */
export function amountKopecks(value: string): number {
  return parseRublesToKopecks(value) ?? 0;
}

/** Экранное значение поля «1234,5» → «1 234,5»: группировка разрядов
 * рублей неразрывным пробелом, копейки как набраны; пустая строка —
 * пустота (плейсхолдера у денежных полей нет). */
export function groupedAmount(value: string): string {
  const [rubles = '', kopecks] = value.split(',');
  const grouped = rubles === '' ? '' : Number(rubles).toLocaleString('ru-RU');
  if (kopecks === undefined) {
    return grouped;
  }
  return `${grouped},${kopecks}`;
}

/**
 * Синхронизация DOM управляемого инпута суммы (общий хвост денежных полей:
 * визарды платежей, компактное поле правки #467, MoneyField аренды): маска
 * могла отбросить символы — если DOM разошёлся с отрисованным значением,
 * пишем отрисованное вручную (React не перерисует совпавший value).
 */
export function syncAmountInputDom(
  event: { value: string },
  sanitized: string,
): void {
  const rendered = groupedAmount(sanitized);
  if (event.value !== rendered) {
    event.value = rendered;
  }
}
