import { describe, expect, it } from 'vitest';
import { mapRental } from './mappers';
import type { components } from '@/shared/api/dto';

type RentalResponseDto = components['schemas']['RentalResponse'];

const DTO: RentalResponseDto = {
  id: '0198c7a2-0000-7000-8000-000000000001',
  propertyId: '0198c7a2-0000-7000-8000-000000000002',
  status: 'active',
  startDate: '2026-09-01',
  plannedEndDate: '2027-08-31',
  completedDate: null,
  utilities: 'included',
  depositKopecks: 3000000,
  commissionKopecks: null,
  depositReturnKopecks: null,
  depositReturnComment: null,
  tenant: {
    contactId: '0198c7a2-0000-7000-8000-000000000003',
    firstName: 'Александр',
    lastName: '',
    phone: '+79001234567',
  },
  comment: '',
  rentPayment: {
    paymentId: '0198c7a2-0000-7000-8000-000000000005',
    amountKopecks: 5600000,
    paymentDay: 10,
    autoPay: false,
    nextPayment: {
      operationId: '0198c7a2-0000-7000-8000-000000000004',
      date: '2026-09-10',
      amountKopecks: 5600000,
      daysUntil: 5,
    },
  },
  progress: { paidMonths: 0, totalMonths: 12, monthsRemaining: 12 },
  today: '2026-09-05',
  createdAt: '2026-09-05T10:00:00Z',
};

describe('mapRental', () => {
  it('маппит полный ответ сервера в сущность', () => {
    expect(mapRental(DTO)).toStrictEqual({
      id: '0198c7a2-0000-7000-8000-000000000001',
      propertyId: '0198c7a2-0000-7000-8000-000000000002',
      status: 'active',
      startDate: '2026-09-01',
      plannedEndDate: '2027-08-31',
      completedDate: null,
      utilities: 'included',
      depositKopecks: 3000000,
      commissionKopecks: null,
      depositReturnKopecks: null,
      depositReturnComment: null,
      tenant: {
        contactId: '0198c7a2-0000-7000-8000-000000000003',
        firstName: 'Александр',
        lastName: '',
        phone: '+79001234567',
      },
      comment: '',
      rentPayment: {
        paymentId: '0198c7a2-0000-7000-8000-000000000005',
        amountKopecks: 5600000,
        paymentDay: 10,
        autoPay: false,
        nextPayment: {
          operationId: '0198c7a2-0000-7000-8000-000000000004',
          date: '2026-09-10',
          amountKopecks: 5600000,
          daysUntil: 5,
        },
      },
      progress: { paidMonths: 0, totalMonths: 12, monthsRemaining: 12 },
      today: '2026-09-05',
      createdAt: '2026-09-05T10:00:00Z',
    });
  });

  it('нормализует отсутствующие в проводе поля в null', () => {
    const rental = mapRental({
      ...DTO,
      plannedEndDate: undefined,
      tenant: undefined,
      rentPayment: { ...DTO.rentPayment, nextPayment: undefined },
    });

    expect(rental.plannedEndDate).toBeNull();
    expect(rental.tenant).toBeNull();
    expect(rental.rentPayment.nextPayment).toBeNull();
  });

  it('читает день оплаты «последний день месяца» и бессрочную аренду', () => {
    const rental = mapRental({
      ...DTO,
      plannedEndDate: null,
      rentPayment: { ...DTO.rentPayment, paymentDay: 'last' },
    });

    expect(rental.rentPayment.paymentDay).toBe('last');
    expect(rental.plannedEndDate).toBeNull();
  });
});
