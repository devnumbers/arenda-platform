import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { propertyPermissions } from './property-permissions';

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

describe('propertyPermissions — центральные права из property.access (#703)', () => {
  it('нет объекта (загрузка/ошибка) — все права закрыты', () => {
    expect(propertyPermissions(undefined)).toEqual({
      canEdit: false,
      canManageMembers: false,
      canLeave: false,
    });
  });

  it('нет контекста доступа — страховка «только чтение»', () => {
    const noAccess: Property = { ...property({ id: 'p1' }), access: undefined };
    expect(propertyPermissions(noAccess)).toEqual({
      canEdit: false,
      canManageMembers: false,
      canLeave: false,
    });
  });

  it('владелец живого объекта — правит, управляет доступом, не покидает свой объект', () => {
    const own = property({ id: 'p1', access: { role: 'owner' } });
    expect(propertyPermissions(own)).toEqual({
      canEdit: true,
      canManageMembers: true,
      canLeave: false,
    });
  });

  it('полный доступ — правит и управляет доступом, может покинуть', () => {
    const editor = property({ id: 'p1', access: { role: 'full_access', ownerName: 'Иван Иванов' } });
    expect(propertyPermissions(editor)).toEqual({
      canEdit: true,
      canManageMembers: true,
      canLeave: true,
    });
  });

  it('зритель — читает: без правок, без управления доступом; покинуть может', () => {
    const watcher = property({ id: 'p1', access: { role: 'viewer', ownerName: 'Иван Иванов' } });
    expect(propertyPermissions(watcher)).toEqual({
      canEdit: false,
      canManageMembers: false,
      canLeave: true,
    });
  });

  it('архив гасит правки, но не управление доступом и не выход (ADR 0028, #446)', () => {
    const archivedOwner = property({ id: 'p1', access: { role: 'owner' }, status: 'archived' });
    expect(propertyPermissions(archivedOwner)).toEqual({
      canEdit: false,
      canManageMembers: true,
      canLeave: false,
    });

    const archivedEditor = property({
      id: 'p2',
      access: { role: 'full_access', ownerName: 'Иван Иванов' },
      status: 'archived',
    });
    expect(propertyPermissions(archivedEditor)).toEqual({
      canEdit: false,
      canManageMembers: true,
      canLeave: true,
    });
  });

  it('объект на ремонте — правки не гасит (мутационный канон гасит только архив)', () => {
    const maintenance = property({ id: 'p1', access: { role: 'owner' }, status: 'maintenance' });
    expect(propertyPermissions(maintenance).canEdit).toBe(true);
  });
});
