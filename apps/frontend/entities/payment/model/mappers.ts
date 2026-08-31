/**
 * DTO → entity модели слайса платежей. Контракты #449 camelCase, поэтому
 * мапперы только нормализуют nullables к опциональности (`?? undefined`)
 * и переименовывают интервалы пауз под доменный язык [from, to).
 */

import type { components } from '@/shared/api/dto';
import type { Payment, PaymentOperation } from './types';

type PaymentDto = components['schemas']['PaymentResponse'];
type OperationDto = components['schemas']['OperationResponse'];

export function mapPayment(dto: PaymentDto): Payment {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    type: dto.type,
    title: dto.title,
    amountKopecks: dto.amountKopecks,
    recurrence:
      dto.recurrence.kind === 'monthly'
        ? {
            kind: 'monthly',
            daysOfMonth: dto.recurrence.daysOfMonth ?? [],
            lastDay: dto.recurrence.lastDay ?? false,
          }
        : dto.recurrence,
    since: dto.since,
    endDate: dto.endDate ?? undefined,
    autoPay: dto.autoPay,
    paymentForm: dto.paymentForm,
    category: {
      source: dto.category.source,
      slug: dto.category.slug ?? undefined,
      id: dto.category.id ?? undefined,
      label: dto.category.label,
    },
    isFavorite: dto.isFavorite,
    isCompleted: dto.isCompleted,
    pauses: dto.pauses.map((pause) => ({
      from: pause.fromDate,
      to: pause.toDate ?? undefined,
    })),
    createdAt: dto.createdAt,
    updatedAt: dto.updatedAt,
  };
}

export function mapPaymentOperation(dto: OperationDto): PaymentOperation {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    paymentId: dto.paymentId,
    date: dto.date,
    paidDate: dto.paidDate ?? undefined,
    status: dto.status,
    type: dto.type,
    title: dto.title,
    amountKopecks: dto.amountKopecks,
    paymentForm: dto.paymentForm ?? undefined,
    categoryLabel: dto.categoryLabel,
    categorySlug: dto.categorySlug ?? undefined,
  };
}
