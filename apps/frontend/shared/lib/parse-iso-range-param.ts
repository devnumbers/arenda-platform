import type { IsoDate, IsoRange } from './calendar';
import { lastDayOfMonth } from './calendar';

/**
 * Чтение диапазона дат из пары query-параметров адреса — канон фильтра
 * периода (экраны операций #477, фильтры истории #711): обе границы
 * обязательны и календарно корректны, перевёрнутый период — null; будущий
 * хвост обрезается «сегодня» (в скоупе лент будущего не бывает, резолюция
 * #474). Минимальный источник параметров — ReadonlyURLSearchParams Next ему
 * удовлетворяет; структурный тип держит модуль чистым для vitest.
 */
export type UrlParamsSource = {
  readonly get: (name: string) => string | null;
};

const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/** Календарно корректная ISO-дата (не «2026-13-40»). */
function isRealIsoDate(iso: string): boolean {
  if (!ISO_DATE_RE.test(iso)) {
    return false;
  }
  const month = Number(iso.slice(5, 7));
  const day = Number(iso.slice(8, 10));
  return month >= 1 && month <= 12 && day >= 1 && day <= lastDayOfMonth(Number(iso.slice(0, 4)), month - 1);
}

export function readIsoRangeParam(
  params: UrlParamsSource,
  fromName: string,
  toName: string,
  today: IsoDate,
): IsoRange | null {
  const rawFrom = params.get(fromName);
  const rawTo = params.get(toName);
  if (rawFrom === null || rawTo === null || !isRealIsoDate(rawFrom) || !isRealIsoDate(rawTo)) {
    return null;
  }
  const to = rawTo < today ? rawTo : today;
  return rawFrom <= to ? { from: rawFrom, to } : null;
}
