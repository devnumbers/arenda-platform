import { describe, expect, it } from 'vitest';
import type { Property } from '@/entities/property';
import { propertyPermissions } from '@/entities/property';
import { propertySectionCta } from './property-section-cta';

function permissions(partial: Partial<Property> & { readonly id: string }) {
  return propertyPermissions({
    name: 'Квартира на Ленина',
    type: 'apartment',
    address: 'Москва, ул. Ленина, 1',
    attributes: {},
    status: 'active',
    members_count: 0,
    created_at: '2026-09-01T10:00:00Z',
    pinned_at: null,
    ...partial,
  });
}

describe('propertySectionCta — кнопка «Добавить» пустой секции детали (#774)', () => {
  it('зритель — CTA не рисуется: формы создания отвечают 403, мёртвых кнопок нет', () => {
    const watcher = permissions({ id: 'p1', access: { role: 'viewer', ownerName: 'Иван Иванов' } });
    expect(propertySectionCta(watcher)).toEqual({ visible: false, disabled: true });
  });

  it('владелец живого объекта — активная кнопка', () => {
    const owner = permissions({ id: 'p1', access: { role: 'owner' } });
    expect(propertySectionCta(owner)).toEqual({ visible: true, disabled: false });
  });

  it('редактор живого объекта — активная кнопка', () => {
    const editor = permissions({
      id: 'p1',
      access: { role: 'full_access', ownerName: 'Иван Иванов' },
    });
    expect(propertySectionCta(editor)).toEqual({ visible: true, disabled: false });
  });

  it('архив гасит canEdit — кнопка владельца остаётся глухой (канон #589, #773)', () => {
    const archivedOwner = permissions({ id: 'p1', access: { role: 'owner' }, status: 'archived' });
    expect(propertySectionCta(archivedOwner)).toEqual({ visible: true, disabled: true });

    const archivedEditor = permissions({
      id: 'p2',
      access: { role: 'full_access', ownerName: 'Иван Иванов' },
      status: 'archived',
    });
    expect(propertySectionCta(archivedEditor)).toEqual({ visible: true, disabled: true });
  });

  it('объект не загружен — прав нет, CTA не рисуется (нет вспышки кнопки)', () => {
    expect(propertySectionCta(propertyPermissions(undefined))).toEqual({
      visible: false,
      disabled: true,
    });
  });

  it('ремонт — не архив: кнопка владельца остаётся активной', () => {
    const maintenance = permissions({ id: 'p1', access: { role: 'owner' }, status: 'maintenance' });
    expect(propertySectionCta(maintenance)).toEqual({ visible: true, disabled: false });
  });
});
