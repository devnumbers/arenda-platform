/**
 * Чистовая модель prefetch-прототипа #610 (исследование): переключатель
 * стратегии, разметка навигационного интента по хабам и троттлинг.
 * Ничего не знает про react-query и роутер — здесь только решения, побочные
 * эффекты в HubPrefetchProvider.
 */

/** Стратегия исследования prefetch (?prefetch=… в адресе; по умолчанию off):
 * off — выключен (базовое поведение), intent — только по навигационному
 * интенту (hover/pointerdown/focus), warm — только прогрев на маунте
 * оболочки, all — и то и другое. */
export type PrefetchStrategy = 'off' | 'intent' | 'warm' | 'all';

const STRATEGIES: ReadonlyArray<PrefetchStrategy> = ['off', 'intent', 'warm', 'all'];

/** Читает стратегию из значения ?prefetch=; отсутствие/незнакомое значение —
 * off: прототип спит до явного включения (флагом или тикетом внедрения). */
export function readPrefetchStrategy(search: string | null): PrefetchStrategy {
  return STRATEGIES.find((value) => value === search) ?? 'off';
}

/** Запись реестра хабов — минимальный контракт модели: префикс маршрута. */
interface HubPrefetchEntryLike {
  readonly prefix: string;
}

/**
 * Ищет хаб по пути из навигационного интента: query/hash отбрасываются,
 * совпадение — по границе пути (внутренние страницы хаба наследуют его
 * префикс, чужие префиксы не совпадают).
 */
export function matchHubPrefix<T extends HubPrefetchEntryLike>(
  entries: ReadonlyArray<T>,
  href: string,
): T | undefined {
  let path = href;
  try {
    path = new URL(href, 'http://localhost').pathname;
  } catch {
    // Относительный путь без схемы URL не нужен: href навигации — '/…'.
  }
  return entries
    .filter((entry) => path === entry.prefix || path.startsWith(`${entry.prefix}/`))
    .toSorted((a, b) => b.prefix.length - a.prefix.length)[0];
}

/**
 * Троттлинг prefetch по навигационному интенту: хаб prefetch'ится не чаще
 * раза в окно — случайные ховеры по сайдбару не долбят бэк (prefetchQuery
 * сам дедуплицирует полёт и уважает staleTime, окно страхует повторные
 * прогревы протухших данных).
 */
export function createPrefetchThrottle(minIntervalMs: number): {
  shouldRun(key: string, now: number): boolean;
} {
  const lastRunAt = new Map<string, number>();
  return {
    shouldRun(key: string, now: number): boolean {
      const last = lastRunAt.get(key);
      if (last !== undefined && now - last < minIntervalMs) {
        return false;
      }
      lastRunAt.set(key, now);
      return true;
    },
  };
}
