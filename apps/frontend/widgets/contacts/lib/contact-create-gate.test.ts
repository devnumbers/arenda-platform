import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';

import { contactCreateGate } from './contact-create-gate';

function property(partial: Partial<Property> & { readonly id: string }): Property {
  return {
    name: 'Квартира на Ленина',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-09-01T10:00:00Z',
    pinned_at: null,
    ...partial,
  };
}

const pending = { isPending: true, isError: false, data: undefined };
const failed = { isPending: false, isError: true, data: undefined };
const loaded = (p: Property) => ({ isPending: false, isError: false, data: p });

describe('contactCreateGate — гейт объектной ветки создания контакта (#509)', () => {
  it('запрос объекта в полёте — скелетон', () => {
    expect(contactCreateGate(pending)).toEqual({ kind: 'pending' });
  });

  it('ошибка загрузки объекта — карточка с «Повторить», а не отказ о правах', () => {
    expect(contactCreateGate(failed)).toEqual({ kind: 'error' });
  });

  it('зритель на загруженном объекте — «только просмотр», не архивная формулировка', () => {
    const watcher = property({ id: 'p1', access: { role: 'viewer', ownerName: 'Иван Иванов' } });
    expect(contactCreateGate(loaded(watcher))).toEqual({
      kind: 'denied',
      archived: false,
    });
  });

  it('архивный объект — архивная формулировка отказа (как в правке контакта)', () => {
    const archivedOwner = property({ id: 'p1', access: { role: 'owner' }, status: 'archived' });
    expect(contactCreateGate(loaded(archivedOwner))).toEqual({
      kind: 'denied',
      archived: true,
    });
  });

  it('владелец живого объекта — форма открывается', () => {
    const owner = property({ id: 'p1', access: { role: 'owner' } });
    expect(contactCreateGate(loaded(owner))).toEqual({ kind: 'allowed' });
  });

  it('полный доступ на живом объекте — форма открывается', () => {
    const editor = property({ id: 'p1', access: { role: 'full_access', ownerName: 'Иван Иванов' } });
    expect(contactCreateGate(loaded(editor))).toEqual({ kind: 'allowed' });
  });
});
