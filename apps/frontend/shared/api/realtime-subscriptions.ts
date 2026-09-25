/**
 * Реестр точных потребителей кадров realtime-стрима (карта #714, тикет
 * #718): экран, которому грубая инвалидация семейства вредит (лента истории
 * — prepend свежих страниц, а не перечитывание окна), подписывается на
 * кадры своей сущности и обрабатывает их сам. Провайдер (features/realtime)
 * читает реестр на каждом кадре: пока у сущности есть живые подписчики,
 * blanket-инвалидация её семейств не выполняется — кадр уходит только
 * подписчикам; без подписчиков провайдер ведёт себя как раньше (ADR 0062
 * §2). Реестр — модульный синглтон вкладки рядом с sse-client: провайдер
 * монтируется раз на сессию, потребители живут в эффектах экранов; слои
 * выше shared связываются структурными типами кадра — словарь имён сущностей
 * остаётся в features/realtime (парсер ADR 0062 §2).
 */

/** Структурный кадр реестра: сущность — строка из словаря (строгость словаря
 * держит парсер features/realtime, реестру достаточно структуры). */
export type RealtimeStreamFrame = {
  readonly entity: string;
  readonly propertyId: string | null;
};

/** Что получает точный потребитель кадра. onOpen подписчикам не доставляется:
 * перечитывание живого на открытии стрима (реплея нет, ADR 0062 §5) делает
 * сама blanket-инвалидация onOpen — смонтированные запросы перечитываются. */
export type RealtimeEntityHandler = {
  readonly onFrame?: (frame: RealtimeStreamFrame) => void;
};

const subscribers = new Map<string, Set<RealtimeEntityHandler>>();

/** Подписывает обработчик на кадры сущности; возвращает отписку. */
export function subscribeRealtimeEntity(
  entity: string,
  handler: RealtimeEntityHandler,
): () => void {
  let handlers = subscribers.get(entity);
  if (handlers === undefined) {
    handlers = new Set<RealtimeEntityHandler>();
    subscribers.set(entity, handlers);
  }
  handlers.add(handler);
  return () => {
    handlers.delete(handler);
  };
}

/** Живые подписчики сущности на момент вызова; пусто — точных потребителей нет. */
export function realtimeEntitySubscribers(entity: string): readonly RealtimeEntityHandler[] {
  return [...(subscribers.get(entity) ?? [])];
}
