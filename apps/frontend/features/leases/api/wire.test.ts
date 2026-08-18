import { describe, expect, it } from 'vitest';
import type { LeaseCreateRequest, LeaseUpdateRequest } from '@/entities/lease';
import { toCreateWireRequest, toUpdateWireRequest } from './hooks';

describe('leases wire serializers', () => {
  it('maps create command fields to snake_case wire keys', () => {
    const command: LeaseCreateRequest = {
      propertyId: 'property-1',
      tenantContactId: 'tenant-1',
      startDate: '2026-01-10',
      endDate: '2026-12-31',
      rentKopecks: 4500000,
      depositKopecks: 9000000,
      paymentDay: 10,
      comment: 'с залогом',
    };

    expect(toCreateWireRequest(command)).toStrictEqual({
      property_id: 'property-1',
      tenant_contact_id: 'tenant-1',
      start_date: '2026-01-10',
      end_date: '2026-12-31',
      rent_amount_kopecks: 4500000,
      deposit_amount_kopecks: 9000000,
      payment_day: 10,
      comment: 'с залогом',
    });
  });

  it('maps update command with tenant clearing to snake_case wire keys', () => {
    const command: LeaseUpdateRequest = {
      tenantContactId: undefined,
      clearTenantContact: true,
      startDate: '2026-02-01',
      endDate: undefined,
      rentKopecks: 5000000,
      depositKopecks: 0,
      paymentDay: 5,
      comment: undefined,
    };

    expect(toUpdateWireRequest(command)).toStrictEqual({
      tenant_contact_id: undefined,
      clear_tenant_contact: true,
      start_date: '2026-02-01',
      end_date: undefined,
      rent_amount_kopecks: 5000000,
      deposit_amount_kopecks: 0,
      payment_day: 5,
      comment: undefined,
    });
  });
});
