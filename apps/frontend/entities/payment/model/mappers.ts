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
  IsoDate,
  OperationsSummary,
  Payment,
  PaymentChangeCategoryRef,
  PaymentChangeEntry,
  PaymentFieldChange,
  PaymentOperation,
  PaymentReminderOffset,
  PaymentType,
  Recurrence,
} from './types';

type PaymentDto = components['schemas']['PaymentResponse'];
type OperationDto = components['schemas']['OperationResponse'];
type OperationsSummaryDto = components['schemas']['OperationsSummaryResponse'];
type GlobalPaymentDto = components['schemas']['PaymentGlobalItem'];
type GlobalPaymentFeedDto = components['schemas']['PaymentsGlobalResponse'];
type GlobalPaymentObjectDto = components['schemas']['PaymentObjectItem'];
type GlobalPaymentSearchDto = components['schemas']['PaymentsSearchGlobalResponse'];
type PaymentChangeItemDto = components['schemas']['PaymentChangeItem'];
type PaymentFieldChangeDto = components['schemas']['PaymentFieldChange'];

/** DTO категории → доменная вью: nullables нормализуются к опциональности.
 * Один хелпер для строки правила и чипа поиска — чип та же CategoryView
 * (#602). */
function mapCategoryView(dto: components['schemas']['CategoryView']) {
  return {
    source: dto.source,
    slug: dto.slug ?? undefined,
    id: dto.id ?? undefined,
    label: dto.label,
  };
}

/** DTO регулярности → доменная форма: у monthly контракт допускает
 * опциональные дни/lastDay, сервер всегда присылает — нормализуем к
 * обязательности канона Recurrence. */
function mapRecurrenceDto(dto: components['schemas']['Recurrence']): Recurrence {
  return dto.kind === 'monthly'
    ? {
        kind: 'monthly',
        daysOfMonth: dto.daysOfMonth ?? [],
        lastDay: dto.lastDay ?? false,
      }
    : dto;
}

export function mapPayment(dto: PaymentDto): Payment {
  return {
    id: dto.id,
    propertyId: dto.propertyId,
    type: dto.type,
    title: dto.title,
    amountKopecks: dto.amountKopecks,
    recurrence: mapRecurrenceDto(dto.recurrence),
    since: dto.since,
    endDate: dto.endDate ?? undefined,
    reminderOffsetDays: dto.reminderOffsetDays ?? undefined,
    autoPay: dto.autoPay,
    notifyAutoPaid: dto.notifyAutoPaid,
    category: mapCategoryView(dto.category),
    isFavorite: dto.isFavorite,
    isCompleted: dto.isCompleted,
    // Не нормализуется к опциональности: явный null («нет даты» — пауза/
    // завершённый) часть семантики поля, см. доку на Payment.nearestDate.
    nearestDate: dto.nearestDate,
    isRentalManaged: dto.isRentalManaged,
    isRentalCompleted: dto.isRentalCompleted,
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
    categoryLabel: dto.categoryLabel,
    categorySlug: dto.categorySlug ?? undefined,
    propertyName: dto.propertyName ?? undefined,
    updatedAt: dto.updatedAt,
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
    category: mapCategoryView(dto.category),
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
    // Чипы — чистые категории: сервер отдаёт одну строку на категорию,
    // направление и счётчик из контракта убраны (#602).
    matchedCategories: dto.matchedCategories.map(mapCategoryView),
    nextCursor: dto.nextCursor ?? null,
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

/** Значение дифа по словарю поля (ADR 0065 §2): схема несёт unknown, словарь
 * гарантирует тип значения под своим ключом — сужаем без рантайм-проверок,
 * контракт-first как весь слой мапперов. Отсутствие значения — null. */
function castChangeValue<T>(value: unknown): T | null {
  return (value ?? null) as T | null;
}

/** Строка дифа журнала изменений: old→new, типизировано ключом поля. */
function mapFieldChange(dto: PaymentFieldChangeDto): PaymentFieldChange {
  switch (dto.field) {
    case 'type':
      return {
        field: 'type',
        old: castChangeValue<PaymentType>(dto.old),
        new: castChangeValue<PaymentType>(dto.new),
      };
    case 'title':
      return { field: 'title', old: castChangeValue<string>(dto.old), new: castChangeValue<string>(dto.new) };
    case 'amount_kopecks':
      return {
        field: 'amount_kopecks',
        old: castChangeValue<number>(dto.old),
        new: castChangeValue<number>(dto.new),
      };
    case 'recurrence':
      return {
        field: 'recurrence',
        old: castChangeValue<Recurrence>(dto.old),
        new: castChangeValue<Recurrence>(dto.new),
      };
    case 'category_slug':
      return {
        field: 'category_slug',
        old: castChangeValue<PaymentChangeCategoryRef>(dto.old),
        new: castChangeValue<PaymentChangeCategoryRef>(dto.new),
      };
    case 'end_date':
      return {
        field: 'end_date',
        old: castChangeValue<IsoDate>(dto.old),
        new: castChangeValue<IsoDate>(dto.new),
      };
    case 'auto_pay':
      return {
        field: 'auto_pay',
        old: castChangeValue<boolean>(dto.old),
        new: castChangeValue<boolean>(dto.new),
      };
    case 'reminder_offset_days':
      return {
        field: 'reminder_offset_days',
        old: castChangeValue<PaymentReminderOffset>(dto.old),
        new: castChangeValue<PaymentReminderOffset>(dto.new),
      };
  }
}

/** Строка журнала изменений платежа (ADR 0065, экран «История платежа»
 * #1195): actor остаётся на DTO — экран актора не показывает. */
export function mapPaymentChangeEntry(dto: PaymentChangeItemDto): PaymentChangeEntry {
  return {
    id: dto.id,
    action: dto.action,
    changes: dto.changes.map(mapFieldChange),
    createdAt: dto.created_at,
  };
}
