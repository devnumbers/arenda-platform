import { realtimeEntitySubscribers } from '@/shared/api/realtime-subscriptions';
import { ENTITY_INVALIDATIONS, REALTIME_FAMILIES } from './entity-invalidations';
import type { RealtimeFrame } from './realtime-frame';
import type { RealtimeStreamHandlers } from './realtime-stream';

/** Обработка разобранного кадра entity.changed (#717): грубый кадр
 * инвалидирует корни семейств query-keys целиком — среза по propertyId
 * кадра нет, перечитывание смонтированных запросов чужих объектов —
 * принятая цена грубости (ADR 0062 §2). Кадры best-effort — стрим никогда
 * не вторая система правды: состояние клиент перечитывает через API. */
/** Срез react-query-клиента, который использует обработчик кадров; полный
 * QueryClient совместим структурно, тесты подсовывают фейк. */
export type StreamQueryClient = {
  invalidateQueries(filter: { queryKey: readonly unknown[] }): Promise<unknown>;
};

export function handleRealtimeFrame(
  frame: RealtimeFrame,
  queryClient: StreamQueryClient,
): void {
  for (const family of ENTITY_INVALIDATIONS[frame.entity]) {
    void queryClient.invalidateQueries({ queryKey: family });
  }
}

/** Открытые-хендлеры стрима: на каждом открытии (включая переподключение и
 * возврат видимости — реплея в v1 нет, ADR 0062 §5) клиент перечитывает
 * живое: инвалидируются все семейства маппинга, смонтированные запросы
 * перечитывают, немонтированные помечаются устаревшими.
 *
 * Кадр сущности, у которой есть живой точный потребитель (реестр
 * realtime-subscriptions, тикет #718), уходит только ему — blanket-инвалидация
 * его семейств подавлена: точный потребитель знает лучше (лента истории
 * prepend'ит свежие страницы, перечитывание окна сдвинуло бы keyset и дёрнуло
 * читающего старые строки). onOpen подавления не имеет: перечитывание на
 * открытии делает сама инвалидация, и после разрыва окно ленты обязано
 * реанкероваться целиком. */
export function realtimeHandlers(queryClient: StreamQueryClient): RealtimeStreamHandlers {
  return {
    onOpen: () => {
      for (const family of REALTIME_FAMILIES) {
        void queryClient.invalidateQueries({ queryKey: family });
      }
    },
    onFrame: (frame) => {
      const subscribers = realtimeEntitySubscribers(frame.entity);
      if (subscribers.length > 0) {
        for (const handler of subscribers) {
          handler.onFrame?.(frame);
        }
        return;
      }
      handleRealtimeFrame(frame, queryClient);
    },
  };
}
