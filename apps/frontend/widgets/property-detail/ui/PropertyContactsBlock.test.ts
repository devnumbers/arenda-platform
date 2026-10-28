import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import type { Contact } from '@/entities/contact';
import { PropertyContactsBlock } from './PropertyContactsBlock';

/** Плоское дерево React-элементов без рендера: канон тестов фронта — node
 * без DOM (CODING_STANDARDS «Testing»); блок без хуков вызывается как
 * функция и инспектируется поэлементно (прецедент
 * contact-section-card.test.ts). Дерево обрывается на hook-компонентах
 * (TenantRowButton, ContactRowButton) — инспектируем их пропы как
 * элементы: строка арендатора узнаётся по props.tenant. */
type InspectableElement = ReactElement<{ children?: ReactNode; [key: string]: unknown }>;

function flatten(node: ReactNode, into: InspectableElement[]): void {
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, into);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as InspectableElement;
  into.push(element);
  flatten(element.props.children, into);
}

function inspect(props: Parameters<typeof PropertyContactsBlock>[0]): InspectableElement[] {
  const elements: InspectableElement[] = [];
  flatten(PropertyContactsBlock(props), elements);
  return elements;
}

function makeContact(overrides: Partial<Contact> = {}): Contact {
  return {
    id: 'contact-1',
    propertyId: 'property-1',
    photoUrl: undefined,
    firstName: 'Анна',
    lastName: '',
    patronymic: '',
    role: '',
    phone: '+79991112233',
    email: '',
    messengerName: '',
    messengerUsername: '',
    note: '',
    createdAt: '2026-10-01T00:00:00Z',
    updatedAt: '2026-10-01T00:00:00Z',
    ...overrides,
  };
}

const TENANT = {
  contactId: 'contact-tenant',
  firstName: 'Максим',
  lastName: '',
  phone: '+79990000000',
} as const;

/** Строка арендатора в дереве — элемент с props.tenant. */
function tenantRowOf(elements: InspectableElement[]): InspectableElement | undefined {
  return elements.find(
    (el) =>
      typeof el.props.tenant === 'object' &&
      el.props.tenant !== null &&
      (el.props.tenant as { contactId?: unknown }).contactId === 'contact-tenant',
  );
}

describe('PropertyContactsBlock — фото арендатора (#1335)', () => {
  it('строка арендатора получает tenantPhotoUrl, даже когда контакта-арендатора нет в списке книги', () => {
    // Страница передаёт список без арендатора (он вырезан, чтобы не
    // дублировать строку) — фото приезжает отдельным пропом из карточки
    // контакта (ADR 0065): поиск по списку всегда давал undefined.
    const row = tenantRowOf(
      inspect({
        tenant: TENANT,
        tenantPhotoUrl: '/api/contacts/contact-tenant/photo',
        tenantPhotoBlur: undefined,
        contacts: [makeContact()],
        onOpenContact: () => {},
      }),
    );

    expect(row).toBeDefined();
    expect(row?.props.photoUrl).toBe('/api/contacts/contact-tenant/photo');
  });

  it('без tenantPhotoUrl строка арендатора получает undefined — аватар-заглушка', () => {
    const row = tenantRowOf(
      inspect({
        tenant: TENANT,
        tenantPhotoUrl: undefined,
        tenantPhotoBlur: undefined,
        contacts: [],
        onOpenContact: () => {},
      }),
    );

    expect(row).toBeDefined();
    expect(row?.props.photoUrl).toBeUndefined();
  });

  it('без арендатора строки арендатора нет, контакты книги рендерятся прежним списком', () => {
    const elements = inspect({
      tenant: null,
      tenantPhotoUrl: undefined,
      tenantPhotoBlur: undefined,
      contacts: [makeContact(), makeContact({ id: 'contact-2', firstName: 'Ольга' })],
      onOpenContact: () => {},
    });

    expect(tenantRowOf(elements)).toBeUndefined();
    const contactRows = elements.filter(
      (el) =>
        typeof el.props.contact === 'object' &&
        el.props.contact !== null &&
        typeof (el.props.contact as { id?: unknown }).id === 'string',
    );
    expect(contactRows).toHaveLength(2);
  });
});
