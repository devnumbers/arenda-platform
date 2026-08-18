import { describe, expect, it } from 'vitest';
import type { components } from '@/shared/api/dto';
import { mapLeaseResponse } from './lease';

type LeaseResponse = components['schemas']['LeaseResponse'];

function makeDto(overrides: Partial<LeaseResponse> = {}): LeaseResponse {
  return {
    id: 'lease-1',
    property_id: 'property-1',
    owner_id: 'owner-1',
    tenant_contact: {
      id: 'tenant-1',
      owner_id: 'owner-1',
      name: 'Иван',
      surname: 'Петров',
      patronymic: null,
      phone: '+79000000000',
      email: null,
      comment: 'предоплата',
      is_active: true,
      active_lease: null,
      last_lease: null,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    status: 'active',
    start_date: '2026-01-10',
    end_date: null,
    rent_amount_kopecks: 4500000,
    deposit_amount_kopecks: 9000000,
    payment_day: 10,
    current_period_overdue: false,
    has_overdue: false,
    overdue_since: null,
    next_payment_date: '2026-02-10',
    comment: 'аренда с залогом',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('mapLeaseResponse', () => {
  it('maps deposit and comment to camelCase fields', () => {
    const lease = mapLeaseResponse(makeDto());

    expect(lease.depositKopecks).toBe(9000000);
    expect(lease.comment).toBe('аренда с залогом');
  });

  it('keeps extended tenant contact with comment', () => {
    const lease = mapLeaseResponse(makeDto());

    expect(lease.tenantContact).toMatchObject({
      id: 'tenant-1',
      name: 'Иван',
      surname: 'Петров',
      phone: '+79000000000',
      comment: 'предоплата',
    });
  });

  it('maps a lease without tenant contact and property', () => {
    const lease = mapLeaseResponse(
      makeDto({ tenant_contact: null, property_id: null, comment: null }),
    );

    expect(lease.tenantContact).toBeNull();
    expect(lease.tenantName).toBe('Арендатор');
    expect(lease.propertyId).toBeNull();
    expect(lease.comment).toBeNull();
  });
});
