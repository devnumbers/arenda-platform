import {
  accessKeys,
  contactKeys,
  globalOperationKeys,
  globalPaymentKeys,
  historyKeys,
  participantsKeys,
  paymentKeys,
  paymentOperationKeys,
  propertyKeys,
  rentalKeys,
  taskKeys,
} from '@/shared/api/query-keys';
import type { RealtimeEntity } from './realtime-frame';

/**
 * Маппинг словаря сущностей стрима на семейства query-keys (карта #714,
 * тикет #717; ADR 0062 §2) — та же таблица контракта, что и на бэке
 * (internal/realtime/domain). Кадры грубые: инвалидируются корни семейств
 * целиком, без среза по propertyId кадра — перечитывание смонтированных
 * запросов чужих объектов — принятая цена грубости (мало объектов на
 * владельца). Одна категория может накрывать несколько семейств —
 * operations накрывает операции платежей и глобальные операции.
 */
export const ENTITY_INVALIDATIONS: Record<RealtimeEntity, ReadonlyArray<readonly unknown[]>> = {
  payments: [paymentKeys.all, globalPaymentKeys.all],
  operations: [paymentOperationKeys.all, globalOperationKeys.all],
  tasks: [taskKeys.all],
  contacts: [contactKeys.all],
  rentals: [rentalKeys.all],
  property: [propertyKeys.all],
  // #719: access-кадры перечитывают и семейства объекта. Получатель
  // гранта/восстановления активен на момент публикации и кадр получает,
  // но его хаб-список объектов и пилюля роли на детали живут в
  // propertyKeys — без этой строки новый объект появился бы в книге
  // только после перезагрузки (таблица ADR 0062 §2).
  access: [accessKeys.all, participantsKeys.all, propertyKeys.all],
  history: [historyKeys.all],
};

/**
 * Все семейства маппинга одним списком — перечитывание живого на открытии
 * стрима (реплея в v1 нет, ADR 0062 §5): на каждом открытии, включая
 * переподключение и возврат видимости, клиент перечитывает всё смонтированное.
 * Выводится из маппинга — новая сущность не требует третьей правки.
 */
export const REALTIME_FAMILIES: ReadonlyArray<readonly unknown[]> = Object.values(
  ENTITY_INVALIDATIONS,
).flat();
