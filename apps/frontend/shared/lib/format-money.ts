/**
 * The canonical money-formatting module: every kopecks↔rubles conversion and
 * every percent scaling in the app goes through these helpers — raw `/100`,
 * `*100` and `.toFixed` arithmetic is banned outside formatting modules
 * (enforced by no-restricted-syntax in eslint.config.mjs, quality bar wave C).
 */

export function formatMoneyKopecks(
  kopecks: number,
  options: { round?: boolean } = {},
): string {
  if (!Number.isFinite(kopecks)) return '';
  const value = options.round ? Math.round(kopecks / 100) : kopecks / 100;
  return `${value.toLocaleString('ru-RU')} ₽`;
}

/**
 * Kopecks → plain decimal string for input fields ("123.45"): two decimals,
 * no digit grouping, no currency symbol — the input counterpart of
 * `formatMoneyKopecks`.
 */
export function kopecksToRublesString(kopecks: number): string {
  return (kopecks / 100).toFixed(2);
}

/**
 * Kopecks → raw input-field string the way the user types it: whole rubles
 * without a decimal part ("145"), fractional amounts with exactly two
 * decimals ("145,50"). The display rule for saved values in amount fields —
 * it never appends ",00" the user did not type (owner note 2026-08-31).
 * Integer operations only.
 */
export function kopecksToAmountInputString(kopecks: number): string {
  const rubles = Math.trunc(kopecks / 100);
  const remainder = kopecks % 100;
  return remainder === 0
    ? String(rubles)
    : `${rubles},${String(remainder).padStart(2, '0')}`;
}

/**
 * Input-field string → kopecks. Accepts dot or comma as the decimal
 * separator, rounds to the nearest kopeck. Returns `undefined` when the value
 * is not a parseable non-negative amount (empty string included); with
 * `positive: true` a zero amount is invalid too.
 */
export function parseRublesToKopecks(
  value: string,
  options: { positive?: boolean } = {},
): number | undefined {
  const normalized = value.trim().replace(',', '.');
  if (normalized === '') {
    return undefined;
  }
  const rubles = Number(normalized);
  if (Number.isNaN(rubles) || rubles < 0 || (options.positive && rubles === 0)) {
    return undefined;
  }
  return Math.round(rubles * 100);
}

/**
 * The explicit spelling of `ratio * 100` for percent widths and positions
 * (progress fills, bar charts): a ratio in 0..1 becomes percent points.
 * Callers own clamping.
 */
export function ratioToPercent(ratio: number): number {
  return ratio * 100;
}
