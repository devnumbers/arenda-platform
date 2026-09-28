import { describe, expect, it } from 'vitest';
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
import { ENTITY_INVALIDATIONS, REALTIME_FAMILIES } from './entity-invalidations';
import { REALTIME_ENTITY_NAMES } from './realtime-frame';

describe('ENTITY_INVALIDATIONS — словарь → семейства query-keys (ADR 0062 §2)', () => {
  it('каждая сущность словаря накрывает свои семейства из таблицы ADR', () => {
    expect(ENTITY_INVALIDATIONS.payments).toStrictEqual([paymentKeys.all, globalPaymentKeys.all]);
    expect(ENTITY_INVALIDATIONS.operations).toStrictEqual([
      paymentOperationKeys.all,
      globalOperationKeys.all,
    ]);
    expect(ENTITY_INVALIDATIONS.tasks).toStrictEqual([taskKeys.all]);
    expect(ENTITY_INVALIDATIONS.contacts).toStrictEqual([contactKeys.all]);
    expect(ENTITY_INVALIDATIONS.rentals).toStrictEqual([rentalKeys.all]);
    expect(ENTITY_INVALIDATIONS.property).toStrictEqual([propertyKeys.all]);
    // #719: access-кадры перечитывают и семейства объекта — грант/восстановление
    // меняют список объектов получателя, смена роли — пилюлю роли на детали;
    // получатель таких переходов активен на момент публикации и кадр получает.
    expect(ENTITY_INVALIDATIONS.access).toStrictEqual([
      accessKeys.all,
      participantsKeys.all,
      propertyKeys.all,
    ]);
    expect(ENTITY_INVALIDATIONS.history).toStrictEqual([historyKeys.all]);
  });

  it('маппинг покрывает ровно словарь — без лишних и без пропусков', () => {
    expect(Object.keys(ENTITY_INVALIDATIONS).sort()).toEqual([...REALTIME_ENTITY_NAMES].sort());
  });

  it('REALTIME_FAMILIES — объединение всех семейств для перечитывания на открытии', () => {
    // propertyKeys входит дважды — из строки property и из строки access
    // (#719): объединение выводится из маппинга как есть, повторная
    // инвалидация того же корня безвредна.
    expect(REALTIME_FAMILIES).toStrictEqual([
      paymentKeys.all,
      globalPaymentKeys.all,
      paymentOperationKeys.all,
      globalOperationKeys.all,
      taskKeys.all,
      contactKeys.all,
      rentalKeys.all,
      propertyKeys.all,
      accessKeys.all,
      participantsKeys.all,
      propertyKeys.all,
      historyKeys.all,
    ]);
  });
});
