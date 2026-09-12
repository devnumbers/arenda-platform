/**
 * DTO → entity модели слайса платежей. Контракты #449 camelCase, поэтому
 * мапперы только нормализуют nullables к опциональности (`?? undefined`)
 * и переименовывают интервалы пауз под доменный язык [from, to).
 */

import type { components } from '@/shared/api/dto';
import type {
  GlobalPayment,
  GlobalPaymentFeed,
  GlobalPaymentObject,
  GlobalPaymentSearch,
  OperationsSummary,
  Payment,
  PaymentOperation,
} from './types';

type PaymentDto = components['schemas']['PaymentResponse'];
type OperationDto = components['schemas']['OperationResponse'];
type OperationsSummaryDto = components['schemas']['OperationsSummaryResponse'];
type GlobalPaymentDto = components['schemas']['PaymentGlobalItem'];
type GlobalPaymentFeedDto = components['schemas']['PaymentsGlobalResponse'];
type GlobalPaymentObjectDto = components['schemas']['PaymentObjectItem'];
type GlobalPaymentSearchDto = components['schemas']['PaymentsSearchGlobalResponse'];

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
    propertyName: dto.propertyName ?? undefined,
  };
}

export function mapOperationsSummary(dto: OperationsSummaryDto): OperationsSummary {
  return {
    incomeTotalKopecks: dto.incomeTotalKopecks,
    expenseTotalKopecks: dto.expenseTotalKopecks,
    categories: dto.categories.map((category) => ({
      slug: category.categorySlug,
      label: category.categoryLabel,
      type: category.type,
      totalKopecks: category.totalKopecks,
    })),
  };
}

/** Строка глобального фида (#575): nullable-поля остаются nullable —
 * «нет значения» здесь часть семантики (нет вхождения / нет просрочки /
 * нет позиции избранного), не опциональность. */
export function mapGlobalPayment(dto: GlobalPaymentDto): GlobalPayment {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    propertyName: dto.propertyName,
    title: dto.title,
    amountKopecks: dto.amountKopecks,
    type: dto.type,
    category: {
      source: dto.category.source,
      slug: dto.category.slug ?? undefined,
      id: dto.category.id ?? undefined,
      label: dto.category.label,
    },
    autoPay: dto.autoPay,
    isFavorite: dto.isFavorite,
    favoriteOrder: dto.favoriteOrder,
    today: dto.today,
    nearestDate: dto.nearestDate,
    overdueOperationCount: dto.overdueOperationCount,
    overdueDays: dto.overdueDays,
    oldestOverdueOperationId: dto.oldestOverdueOperationId,
  };
}

export function mapGlobalPaymentFeed(dto: GlobalPaymentFeedDto): GlobalPaymentFeed {
  return {
    items: dto.items.map(mapGlobalPayment),
    favoriteCount: dto.favoriteCount,
    overdueOperationsCount: dto.overdueOperationsCount,
  };
}

export function mapGlobalPaymentSearch(dto: GlobalPaymentSearchDto): GlobalPaymentSearch {
  return {
    items: dto.items.map(mapGlobalPayment),
    nextCursor: dto.nextCursor ?? null,
    matchedCategories: dto.matchedCategories.map((chip) => ({
      category: {
        source: chip.category.source,
        slug: chip.category.slug ?? undefined,
        id: chip.category.id ?? undefined,
        label: chip.category.label,
      },
      type: chip.type,
      count: chip.count,
    })),
  };
}

export function mapGlobalPaymentObject(dto: GlobalPaymentObjectDto): GlobalPaymentObject {
  const toKey = (key: GlobalPaymentObjectDto['autoPayRules'][number]) => ({
    paymentId: key.paymentId,
    hasOverdue: key.hasOverdue,
  });

  return {
    propertyId: dto.propertyId,
    name: dto.name,
    address: dto.address,
    pinnedAt: dto.pinnedAt,
    photoUrl: dto.photoUrl,
    autoPayRules: dto.autoPayRules.map(toKey),
    otherRules: dto.otherRules.map(toKey),
  };
}
