/**
 * Чистое ядро сборки и патча адреса (pre-merge #785; дом — находка
 * повторного pre-merge #785: правила нужны сервер-безопасным потребителям,
 * а в 'use client'-хуке use-url-params они тянули next/navigation
 * транзитивно — корень фоллоу-апов #792). Писатель с режимами
 * replace/push — хук use-url-params; поведенческие гарантии — в
 * url-params.test.ts.
 */

/**
 * Патч полного желаемого состояния группы `own` поверх текущих параметров:
 * собственные ключи `own` снимаются перед set, поэтому ключ группы без
 * значения в patch (дефолт) удаляется из адреса, а не остаётся протухшим —
 * урок фикса 0c64fcc3; чужие параметры (например, фильтр ленты задач
 * #524) не трогаются.
 */
export function applyUrlParamsPatch(
  current: URLSearchParams,
  patch: Record<string, string>,
  own: ReadonlyArray<string>,
): URLSearchParams {
  const next = new URLSearchParams(current);
  for (const name of own) {
    next.delete(name);
  }
  for (const [name, value] of Object.entries(patch)) {
    next.set(name, value);
  }
  return next;
}

/** Адрес с query или без (пустые параметры — голый pathname). */
export function buildUrlWithParams(pathname: string, params: URLSearchParams): string {
  const query = params.toString();
  return query.length > 0 ? `${pathname}?${query}` : pathname;
}
