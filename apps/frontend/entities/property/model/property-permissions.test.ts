import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { filterEditableProperties, filterOwnedProperties, propertyPermissions } from './property-permissions';

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
      canView: false,
      canMutate: false,
      canManageMembers: false,
      canLeave: false,
      canLifecycle: false,
    });
  });

  it('нет контекста доступа — страховка «только чтение»', () => {
    const noAccess: Property = { ...property({ id: 'p1' }), access: undefined };
    expect(propertyPermissions(noAccess)).toEqual({
      canEdit: false,
      canView: false,
      canMutate: false,
      canManageMembers: false,
      canLeave: false,
      canLifecycle: false,
    });
  });

  it('владелец живого объекта — читает, правит, мутации, управляет участниками и жизненным циклом, не покидает свой объект', () => {
    const own = property({ id: 'p1', access: { role: 'owner' } });
    expect(propertyPermissions(own)).toEqual({
      canEdit: true,
      canView: true,
      canMutate: true,
      canManageMembers: true,
      canLeave: false,
      canLifecycle: true,
    });
  });

  it('полный доступ — читает и правит, но участниками управляет только собственник (D8 #1304); покинуть может; жизненный цикл владельческий (#757)', () => {
    const editor = property({ id: 'p1', access: { role: 'full_access', ownerName: 'Иван Иванов' } });
    expect(propertyPermissions(editor)).toEqual({
      canEdit: true,
      canView: true,
      canMutate: true,
      canManageMembers: false,
      canLeave: true,
      canLifecycle: false,
    });
  });

  it('зритель — читает: без правок, без мутаций, без управления доступом; покинуть может', () => {
    const watcher = property({ id: 'p1', access: { role: 'viewer', ownerName: 'Иван Иванов' } });
    expect(propertyPermissions(watcher)).toEqual({
      canEdit: false,
      canView: true,
      canMutate: false,
      canManageMembers: false,
      canLeave: true,
      canLifecycle: false,
    });
  });

  it('архив гасит правки, но не ролевые мутации, управление участниками и выход (ADR 0028, #446)', () => {
    const archivedOwner = property({ id: 'p1', access: { role: 'owner' }, status: 'archived' });
    expect(propertyPermissions(archivedOwner)).toEqual({
      canEdit: false,
      canView: true,
      canMutate: true,
      canManageMembers: true,
      canLeave: false,
      canLifecycle: true,
    });

    const archivedEditor = property({
      id: 'p2',
      access: { role: 'full_access', ownerName: 'Иван Иванов' },
      status: 'archived',
    });
    expect(propertyPermissions(archivedEditor)).toEqual({
      canEdit: false,
      canView: true,
      canMutate: true,
      canManageMembers: false,
      canLeave: true,
      canLifecycle: false,
    });
  });

  it('объект на ремонте — правки не гасит (мутационный канон гасит только архив)', () => {
    const maintenance = property({ id: 'p1', access: { role: 'owner' }, status: 'maintenance' });
    expect(propertyPermissions(maintenance).canEdit).toBe(true);
  });
});

describe('фильтры справочника объектов для пикеров', () => {
  const own = property({ id: 'own', access: { role: 'owner' } });
  const editable = property({ id: 'editable', access: { role: 'full_access', ownerName: 'Иван Иванов' } });
  const watched = property({ id: 'watched', access: { role: 'viewer', ownerName: 'Иван Иванов' } });
  const archivedOwn = property({ id: 'archived', access: { role: 'owner' }, status: 'archived' });
  const book = [own, editable, watched, archivedOwn];

  it('filterEditableProperties — полный доступ правит: своя и редактируемая, без зрительской и архивной (#703)', () => {
    expect(filterEditableProperties(book).map((p) => p.id)).toEqual(['own', 'editable']);
  });

  it('filterOwnedProperties — только собственные живые: перепривязка — власть собственника (D10 #1309)', () => {
    expect(filterOwnedProperties(book).map((p) => p.id)).toEqual(['own']);
  });
});
