import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import type { Contact } from '@/entities/contact';
import { UserAvatar } from '@/shared/ui/design';
import { ContactDetailHeader } from './contact-detail-header';

/** Плоское дерево React-элементов без рендера: канон тестов фронта — node
 * без DOM (CODING_STANDARDS «Testing»); шапка без хуков вызывается как
 * функция и инспектируется поэлементно (прецедент phone-step.test.ts). */
type InspectableElement = ReactElement<{ children?: ReactNode; [key: string]: unknown }>;

function flatten(node: ReactNode, into: InspectableElement[], strings: string[]): void {
  if (typeof node === 'string') {
    strings.push(node);
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, into, strings);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as InspectableElement;
  into.push(element);
  flatten(element.props.children, into, strings);
}

function inspect(props: Parameters<typeof ContactDetailHeader>[0]): {
  elements: InspectableElement[];
  strings: string[];
} {
  const elements: InspectableElement[] = [];
  const strings: string[] = [];
  flatten(ContactDetailHeader(props), elements, strings);
  return { elements, strings };
}

const baseContact: Contact = {
  id: '0198b6a7-1000-7000-8000-0000000000c1',
  propertyId: undefined,
  firstName: 'Александр',
  lastName: 'Иванов',
  patronymic: 'Сергеевич',
  role: 'Арендатор',
  phone: '+79931231212',
  email: 'email@yandex.ru',
  messengerName: 'Телеграм',
  messengerUsername: 'username',
  note: '',
  createdAt: '2026-09-01T09:00:00Z',
  updatedAt: '2026-09-02T10:30:00Z',
};

describe('ContactDetailHeader — ФИО и роль на фоне страницы (макет 3304:75726)', () => {
  it('ФИО целиком и роль под ним, фото уходит в аватар', () => {
    const { elements, strings } = inspect({
      contact: baseContact,
      photoUrl: '/api/contacts/c1/photo?v=2',
    });

    expect(strings).toContain('Александр Иванов Сергеевич');
    expect(strings).toContain('Арендатор');
    const avatar = elements.find((el) => el.type === UserAvatar);
    expect(avatar?.props.size).toBe('hero');
    expect(avatar?.props.photoUrl).toBe('/api/contacts/c1/photo?v=2');
  });

  it('роль пуста — строки роли нет (решение при чартинге: видна, когда заполнена)', () => {
    const { strings } = inspect({
      contact: { ...baseContact, role: '' },
      photoUrl: null,
    });

    expect(strings).toContain('Александр Иванов Сергеевич');
    expect(strings).not.toContain('Арендатор');
  });
});
