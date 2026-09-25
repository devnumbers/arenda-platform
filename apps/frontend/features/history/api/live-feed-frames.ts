import type { RealtimeStreamFrame } from '@/shared/api/realtime-subscriptions';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';

/**
 * Обработчик кадров history для живой ленты (карта #714, тикет #718;
 * ADR 0062 §2, точный потребитель поверх ADR 0062 §5): кадр «у объекта
 * были записи журнала» догоняет ленту СНИЗУ — prepend свежих страниц по
 * стороне prev двустороннего keyset (#709: «сам prepend свежих подключит
 * realtime-карта») — вместо перечитывания окна, которое сдвинуло бы
 * keyset-границы и дёрнуло читающего старые строки.
 *
 * Правила:
 * - кадр по объекту вне скоупа ленты (прибитая страница #840/#841, фильтр
 *   объектов #711) игнорируется; кадр без объекта проходит всегда;
 * - prepend'ы сериализуются цепочкой промисов — параллельные prepend'ы с
 *   одной свежей границей надублировали бы строки;
 * - prepend догоняет страницу за страницей, пока свежая граница страницы
 *   не упрётся в дно (prevCursor null): за один кадр мог накопиться целый
 *   burst; после потолка — перечитывание окна целиком (реанкер, редчайший
 *   случай >250 записей за разрыв);
 * - нет свежей границы (лента пуста — prepend'ить не от чего) — кадр
 *   перечитывает ленту целиком;
 * - неготовый запрос (первая загрузка, скоуп «в ноль») кадр пропускает —
 *   загрузка сама свежая, перечитывание сделает onOpen-инвалидация.
 *
 * Ошибки prepend'а/перечитывания глокаются: кадры best-effort (ADR 0062),
 * следующий кадр или открытие стрима догонит.
 */

/** Потолок доprepend'ов на один кадр: 5 страниц по 50 записей покрывают
 * burst за разрыв; сверх — перечитывание окна честнее длинной очереди. */
const CATCH_UP_MAX_PREPENDS = 5;

export type LiveFeedState = {
  /** Запрос готов принимать prepend (включён и имеет загруженные страницы). */
  readonly enabled: boolean;
  /** Есть свежая граница (непустая лента несёт prevCursor — контракт #708). */
  readonly hasPreviousPage: boolean;
  /** Фильтр объектов скоупа (#711) или пин страницы (#840/#841); undefined —
   * лента всей области чтения. */
  readonly scopePropertyIds: ReadonlyArray<string> | undefined;
};

export type LiveFeedDeps = {
  /** Снимок состояния запроса на момент кадра. */
  readonly state: () => LiveFeedState;
  /** fetchPreviousPage; результат несёт курсор свежей границы ПЕРВОЙ
   * (новой) страницы после prepend — null значит «дно достигнуто». */
  readonly prepend: () => Promise<{ firstPrevCursor: string | null } | null>;
  /** Полное перечитывание ленты. */
  readonly refetch: () => Promise<unknown>;
};

export function createLiveFeedFrameHandler(deps: LiveFeedDeps): (frame: RealtimeStreamFrame) => void {
  // Сериализация — цепочкой промисов: prepend'ы двух кадров никогда не
  // летят параллельно (одна свежая граница надублировала бы строки), а
  // догон после burst'а делает сам цикл прогона — до границы «дно».
  let tail: Promise<void> = Promise.resolve();

  const run = async (): Promise<void> => {
    const { enabled, hasPreviousPage } = deps.state();
    if (!enabled) {
      return;
    }
    if (!hasPreviousPage) {
      await deps.refetch();
      return;
    }
    for (let page = 0; page < CATCH_UP_MAX_PREPENDS; page += 1) {
      const result = await deps.prepend();
      if (result === null || result.firstPrevCursor === null) {
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
