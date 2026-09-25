import type { RealtimeStreamFrame } from '@/shared/api/realtime-subscriptions';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';

/**
 * Обработчик кадров history для живой ленты (карта #714, тикет #718;
 * ADR 0062 §2, точный потребитель поверх ADR 0062 §5): кадр «у объекта
 * были записи журнала» догоняет ленту СНИЗУ — свежие строки вливаются в
 * первую страницу кэша (#709: «сам prepend свежих подключит realtime-карта»)
 * — вместо перечитывания окна, которое сдвинуло бы keyset-границы и дёрнуло
 * читающего старые строки.
 *
 * Правила:
 * - кадр по объекту вне скоупа ленты (прибитая страница #840/#841, фильтр
 *   объектов #711) игнорируется; кадр без объекта проходит всегда;
 * - догон сериализуется цепочкой промисов — параллельные догоны с одной
 *   границей надублировали бы строки;
 * - догон страница за страницей, пока порция ложится ровно в pageSize
 *   (есть что дочитывать); «дно» — неполная порция: по курсору дно не
 *   детектится (prev_cursor есть у любой непустой страницы — контракт #708);
 *   после потолка страниц — перечитывание окна целиком (реанкер, редчайший
 *   burst >250 записей за разрыв);
 * - нет загруженных строк (лента пуста — вливать не во что) — кадр
 *   перечитывает ленту целиком;
 * - неготовый запрос (первая загрузка, скоуп «в ноль») кадр пропускает —
 *   загрузка сама свежая, перечитывание сделает onOpen-инвалидация.
 *
 * Ошибки догона/перечитывания репортятся и глотаются: кадры best-effort
 * (ADR 0062), следующий кадр или открытие стрима догонит.
 */

/** Потолок страниц догона на один кадр: 5 порций по 50 записей покрывают
 * burst за разрыв; сверх — перечитывание окна честнее длинной очереди. */
const CATCH_UP_MAX_PREPENDS = 5;

export type LiveFeedState = {
  /** Запрос готов принимать догон (включён и отработал первую загрузку). */
  readonly enabled: boolean;
  /** Есть загруженные строки — к границе можно вливать свежие. */
  readonly hasPages: boolean;
  /** Фильтр объектов скоупа (#711) или пин страницы (#840/#841); undefined —
   * лента всей области чтения. */
  readonly scopePropertyIds: ReadonlyArray<string> | undefined;
};

export type LiveFeedDeps = {
  /** Снимок состояния запроса на момент кадра. */
  readonly state: () => LiveFeedState;
  /** Догоняет снизу: читает записи моложе загруженной границы и вливает их
   * в первую страницу кэша; hasMore — порция легла ровно в pageSize
   * (возможны ещё свежие). null — догонять не из чего (нет границы). */
  readonly fetchFresh: () => Promise<{ hasMore: boolean } | null>;
  /** Полное перечитывание ленты. */
  readonly refetch: () => Promise<unknown>;
};

export function createLiveFeedFrameHandler(deps: LiveFeedDeps): (frame: RealtimeStreamFrame) => void {
  // Сериализация — цепочкой промисов: догоны двух кадров никогда не летят
  // параллельно (одна граница надублировала бы строки).
  let tail: Promise<void> = Promise.resolve();

  const run = async (): Promise<void> => {
    const { enabled, hasPages } = deps.state();
    if (!enabled) {
      return;
    }
    if (!hasPages) {
      await deps.refetch();
      return;
    }
    for (let page = 0; page < CATCH_UP_MAX_PREPENDS; page += 1) {
      const result = await deps.fetchFresh();
      if (result === null || !result.hasMore) {
        return;
      }
    }
    await deps.refetch();
  };

  return (frame: RealtimeStreamFrame) => {
    const { scopePropertyIds } = deps.state();
    if (
      frame.propertyId !== null
      && scopePropertyIds !== undefined
      && !scopePropertyIds.includes(frame.propertyId)
    ) {
      return;
    }
    tail = tail
      .then(run)
      .catch((error: unknown) => {
        // Кадры best-effort (ADR 0062): неготовый запрос/сеть — следующий
        // кадр или onOpen-перечитывание догонит; сбой репортится (лимит
        // репортер глушит сам).
        reportClientError(`history live prepend failed: ${String(error)}`);
      });
  };
}
