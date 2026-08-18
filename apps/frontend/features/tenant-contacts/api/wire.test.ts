import { describe, expect, it } from 'vitest';
import type {
  TenantContactCreateRequest,
  TenantContactUpdateRequest,
} from '@/entities/tenant-contact';
import { toCreateWireRequest, toUpdateWireRequest } from './hooks';

describe('tenant-contacts wire serializers', () => {
  it('maps propertyId to property_id on create', () => {
    const command: TenantContactCreateRequest = {
      name: 'Иван',
      surname: 'Петров',
      patronymic: undefined,
      phone: '+79000000000',
      email: undefined,
      comment: 'предоплата',
      propertyId: 'property-1',
    };

    expect(toCreateWireRequest(command)).toStrictEqual({
      name: 'Иван',
      surname: 'Петров',
      patronymic: undefined,
      phone: '+79000000000',
      email: undefined,
      comment: 'предоплата',
      property_id: 'property-1',
    });
  });

  it('maps update command fields without extra keys', () => {
    const command: TenantContactUpdateRequest = {
      name: 'Иван',
      surname: undefined,
      patronymic: 'Сергеевич',
      phone: undefined,
      email: 'ivan@example.com',
      comment: undefined,
    };

    expect(toUpdateWireRequest(command)).toStrictEqual({
      name: 'Иван',
      surname: undefined,
      patronymic: 'Сергеевич',
      phone: undefined,
      email: 'ivan@example.com',
      comment: undefined,
    });
  });
});
