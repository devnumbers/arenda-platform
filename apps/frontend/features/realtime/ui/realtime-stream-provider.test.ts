import { describe, expect, it, vi } from 'vitest';
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
import { subscribeRealtimeEntity } from '@/shared/api/realtime-subscriptions';
import { REALTIME_FAMILIES } from '../api/entity-invalidations';
import type { RealtimeFrame } from '../api/realtime-frame';
import { handleRealtimeFrame, realtimeHandlers, type StreamQueryClient } from './realtime-stream-provider';

/** Фейк QueryClient структурного среза обработчика: фиксирует инвалидации —
 * без vi.fn, сигнатура совпадает с Pick<QueryClient…>. */
function fakeQueryClient() {
  const invalidated: ReadonlyArray<unknown>[] = [];
  const client: StreamQueryClient = {
    invalidateQueries: ({ queryKey }: { queryKey: ReadonlyArray<unknown> }): Promise<unknown> => {
      invalidated.push(queryKey);
      return Promise.resolve();
    },
  };
  return { ...client, invalidated };
}

function frame(entity: RealtimeFrame['entity'], propertyId: string | null = 'p1'): RealtimeFrame {
  return { entity, propertyId };
}

describe('handleRealtimeFrame — грубый кадр → инвалидация семейств (ADR 0062 §2)', () => {
  it('payments инвалидирует платежи объекта и глобальные платежи', () => {
    const qc = fakeQueryClient();
    handleRealtimeFrame(frame('payments'), qc);
    expect(qc.invalidated).toStrictEqual([paymentKeys.all, globalPaymentKeys.all]);
  });

  it('operations инвалидирует операции платежей и глобальные операции', () => {
    const qc = fakeQueryClient();
    handleRealtimeFrame(frame('operations'), qc);
    expect(qc.invalidated).toStrictEqual([paymentOperationKeys.all, globalOperationKeys.all]);
  });

  it('access инвалидирует доступ к объекту и хаб участников', () => {
    const qc = fakeQueryClient();
    handleRealtimeFrame(frame('access'), qc);
    expect(qc.invalidated).toStrictEqual([accessKeys.all, participantsKeys.all]);
  });

  it('односемейные сущности инвалидируют свой корень; propertyId кадра не сужает', () => {
    const qc = fakeQueryClient();
    handleRealtimeFrame(frame('tasks'), qc);
    handleRealtimeFrame(frame('contacts'), qc);
    handleRealtimeFrame(frame('rentals'), qc);
    handleRealtimeFrame(frame('property'), qc);
    handleRealtimeFrame(frame('history'), qc);
    expect(qc.invalidated).toStrictEqual([
      taskKeys.all,
      contactKeys.all,
      rentalKeys.all,
      propertyKeys.all,
      historyKeys.all,
    ]);
  });

  it('безобъектная книга (propertyId null) инвалидирует так же — автор сам участник кадра', () => {
    const qc = fakeQueryClient();
    handleRealtimeFrame(frame('operations', null), qc);
    expect(qc.invalidated).toStrictEqual([paymentOperationKeys.all, globalOperationKeys.all]);
  });
});

describe('realtimeHandlers — перечитывание живого на открытии (реплея нет)', () => {
  it('onOpen инвалидирует все семейства маппинга разом', () => {
    const qc = fakeQueryClient();
    realtimeHandlers(qc).onOpen?.();
    expect(qc.invalidated).toStrictEqual([...REALTIME_FAMILIES]);
  });

  it('onFrame диспатчит в handleRealtimeFrame', () => {
    const qc = fakeQueryClient();
    realtimeHandlers(qc).onFrame?.(frame('history'));
    expect(qc.invalidated).toStrictEqual([historyKeys.all]);
  });
});

describe('кадр сущности с точным потребителем (тикет #718)', () => {
  it('пока подписчик жив — blanket-инвалидации нет, кадр уходит только ему', () => {
    const qc = fakeQueryClient();
    const onFrame = vi.fn();
    const unsubscribe = subscribeRealtimeEntity('history', { onFrame });

    realtimeHandlers(qc).onFrame?.(frame('history'));
    realtimeHandlers(qc).onFrame?.(frame('payments'));

    expect(onFrame).toHaveBeenCalledExactlyOnceWith(frame('history'));
    expect(qc.invalidated).toStrictEqual([paymentKeys.all, globalPaymentKeys.all]);
    unsubscribe();
  });

  it('после отписки сущность снова инвалидируется как раньше', () => {
    const qc = fakeQueryClient();
    const unsubscribe = subscribeRealtimeEntity('history', {});

    unsubscribe();
    realtimeHandlers(qc).onFrame?.(frame('history'));

    expect(qc.invalidated).toStrictEqual([historyKeys.all]);
  });

  it('onOpen перечитывает все семейства, включая сущности с подписчиками', () => {
    const qc = fakeQueryClient();
    const unsubscribe = subscribeRealtimeEntity('history', {});

    realtimeHandlers(qc).onOpen?.();

    expect(qc.invalidated).toStrictEqual([...REALTIME_FAMILIES]);
    unsubscribe();
  });
});
