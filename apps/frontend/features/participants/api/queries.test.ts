import { beforeEach, describe, expect, it, vi } from 'vitest';

// Транспорт подменяется стабом: тест пинит пару ключ+fetch фабрики списка
// (канон #887) и маппинг ответа в сущности. Рендер хука в node-окружении
// недоступен — функции зовутся напрямую.
vi.mock('@/shared/api/client', () => ({ apiClient: vi.fn() }));

import { apiClient } from '@/shared/api/client';
import type { components } from '@/shared/api/dto';
import { participantsKeys } from '@/shared/api/query-keys';
import type { Participant } from '@/entities/participants';
import { fetchParticipantsList, participantsListQueryOptions } from './queries';

const apiClientMock = vi.mocked(apiClient);

type ParticipantDto = components['schemas']['ParticipantResponse'];

function makeDto(
  overrides: Partial<ParticipantDto> = {},
): ParticipantDto {
  return {
    id: 'user-1',
    user_id: '11111111-1111-4111-8111-111111111111',
    email: 'e2e@example.com',
    display_name: 'Иван Иванов',
    photo_url: null,
    aggregate_status: 'all_properties',
    accessible_properties_count: 2,
    properties: [
      {
        property_id: '44444444-4444-4444-8444-444444444444',
        title: 'Гараж на Садовой',
        role: 'viewer',
        status: 'active',
        type: 'garage',
        photo_url: null,
      },
    ],
    ...overrides,
  };
}

describe('participantsListQueryOptions — пара ключ+fetch списка (канон #887)', () => {
  it('ключ — реестр participantsKeys.list, fetch — функция', () => {
    const options = participantsListQueryOptions();

    expect(options.queryKey).toStrictEqual(participantsKeys.list());
    expect(typeof options.queryFn).toBe('function');
  });

  it('транспорт прокидывается в fetch-функцию (серверный префетч)', async () => {
    const transport = vi.fn().mockResolvedValue({ items: [] });
    const options = participantsListQueryOptions({ transport });

    expect(typeof options.queryFn).toBe('function');
    const participants = await (options.queryFn as () => Promise<Participant[]>)();

    expect(transport).toHaveBeenCalledTimes(1);
    expect(transport).toHaveBeenCalledWith('/participants');
    expect(apiClientMock).not.toHaveBeenCalled();
    expect(participants).toStrictEqual([]);
  });
});

describe('fetchParticipantsList — GET /participants', () => {
  beforeEach(() => {
    apiClientMock.mockReset();
  });

  it('делегирует в транспорт списочным путём, ответ маппится в сущности', async () => {
    apiClientMock.mockResolvedValue({ items: [makeDto()] });

    const participants = await fetchParticipantsList();

    expect(apiClientMock).toHaveBeenCalledTimes(1);
    expect(apiClientMock).toHaveBeenCalledWith('/participants');
    expect(participants).toHaveLength(1);
    expect(participants[0]).toMatchObject({
      id: 'user-1',
      userId: '11111111-1111-4111-8111-111111111111',
      displayName: 'Иван Иванов',
      aggregateStatus: 'all_properties',
      accessiblePropertiesCount: 2,
      properties: [
        {
          propertyId: '44444444-4444-4444-8444-444444444444',
          title: 'Гараж на Садовой',
          role: 'viewer',
          status: 'active',
          type: 'garage',
          photoUrl: null,
        },
      ],
    });
  });
});
