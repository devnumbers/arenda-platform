import { describe, expect, it } from 'vitest';

import type { components } from '@/shared/api/dto';

import { mapHistoryItem, mapHistoryFilterOptions } from './mappers';

type HistoryItemDto = components['schemas']['HistoryItem'];
type HistoryFiltersDto = components['schemas']['HistoryFiltersResponse'];

function itemDto(overrides: Partial<HistoryItemDto> = {}): HistoryItemDto {
  return {
    id: 'entry-1',
    property_id: 'property-1',
    property_name: 'Моя квартира',
    actor_id: 'user-1',
    actor_name: 'Иван Иванов',
    actor_email: 'ivan@example.com',
    actor_role: 'owner',
    kind: 'payment',
    action: 'payment.created',
    base_action: 'added',
    segments: [{ text: 'Платёж создан: ' }, { text: 'Аренда', link: { kind: 'payment', id: 'p-1' } }],
    context: {},
    created_at: '2026-09-22T10:00:00Z',
    ...overrides,
  };
}

describe('mapHistoryItem', () => {
  it('переводит снимок записи в camelCase, сегменты — дословно', () => {
    const entry = mapHistoryItem(itemDto());
    expect(entry).toMatchObject({
      id: 'entry-1',
      propertyId: 'property-1',
      propertyName: 'Моя квартира',
      actorId: 'user-1',
      actorName: 'Иван Иванов',
      actorRole: 'owner',
      baseAction: 'added',
      action: 'payment.created',
      createdAt: '2026-09-22T10:00:00Z',
    });
    expect(entry.segments).toEqual([
      { text: 'Платёж создан: ' },
      { text: 'Аренда', link: { kind: 'payment', id: 'p-1' } },
    ]);
  });

  it('обезличенная запись: actor_id null сохраняется', () => {
    const entry = mapHistoryItem(itemDto({ actor_id: null }));
    expect(entry.actorId).toBeNull();
  });

  it('фрагмент без ссылки остаётся без ссылки', () => {
    const entry = mapHistoryItem(itemDto({ segments: [{ text: 'Описание обновлено' }] }));
    expect(entry.segments).toEqual([{ text: 'Описание обновлено' }]);
  });
});

describe('mapHistoryFilterOptions', () => {
  it('переводит опции шита в camelCase вместе с first_name/is_owner (#711)', () => {
    const options = mapHistoryFilterOptions({
      participants: [
        {
          id: 'user-1',
          name: 'Иван Иванов',
          email: 'ivan@example.com',
          first_name: 'Иван',
          is_owner: true,
        },
      ],
      objects: [
        { id: 'property-1', name: 'Моя квартира', address: 'Ленина 1', photo_url: 'https://cdn/x.jpg' },
        { id: 'property-2', name: 'Гараж', address: '', photo_url: '' },
      ],
    } satisfies HistoryFiltersDto);
    expect(options.participants).toEqual([
      {
        id: 'user-1',
        name: 'Иван Иванов',
        email: 'ivan@example.com',
        firstName: 'Иван',
        isOwner: true,
      },
    ]);
    expect(options.objects).toEqual([
      { id: 'property-1', name: 'Моя квартира', address: 'Ленина 1', photoUrl: 'https://cdn/x.jpg' },
      { id: 'property-2', name: 'Гараж', address: '', photoUrl: '' },
    ]);
  });
});
