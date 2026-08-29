'use client';

import type { Dispatch, SetStateAction } from 'react';
import { clearDraftStorage, useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import type { IsoDate, PaymentForm, PaymentType, Recurrence } from '@/entities/payment';

/**
 * Черновик визарда создания платежа (спека #453, история 10): переживает
 * перезагрузку страницы и закрытие визарда — хранится в localStorage
 * (`storage: 'local'`), ключ per объект+тип выбора из шита («Платёж /
 * Автоплатёж»). Обёртка владеет только ключом, дефолтом и проверкой формы —
 * сам цикл хранения живёт в useDraftStore. Форма черновика покрывает шаги
 * визарда; шаги UI (номер шага и переходы) — слайс визарда, следующего тикета.
 */

/** Тип создаваемого из шита: обычный платёж или автоплатёж (флаг autoPay). */
export type PaymentDraftType = 'payment' | 'autopayment';

export type PaymentWizardDraft = {
  /** Доход/расход — чип шага суммы. */
  readonly type?: PaymentType;
  /** Слаг дефолтного каталога (шаг 1). */
  readonly categorySlug?: string;
  /** Название платежа (шаг 2; пустое — не черновик). */
  readonly title?: string;
  /** Периодичность без «Один раз» (шаг 3). */
  readonly recurrence?: Recurrence;
  /** Окончание платежа (шаг 4). */
  readonly endDate?: IsoDate;
  /** Сумма в копейках, целая положительная (шаг 5). */
  readonly amountKopecks?: number;
  /** Форма оплаты: перевод/наличные (шаг 5). */
  readonly paymentForm?: PaymentForm;
  /** Момент последней правки (Date.now()) — шит «Добавить» показывает
   * последний тронутый черновик, когда их два. Служебное поле: само по
   * себе черновиком не считается. */
  readonly updatedAt?: number;
};

const DEFAULT_DRAFT: PaymentWizardDraft = {};

export function paymentDraftStorageKey(propertyId: string, type: PaymentDraftType): string {
  return `payment-wizard-draft:${propertyId}:${type}`;
}

export function clearPaymentWizardDraft(propertyId: string, type: PaymentDraftType): void {
  clearDraftStorage(paymentDraftStorageKey(propertyId, type), 'local');
}

export function clearPaymentWizardDrafts(propertyId: string): void {
  clearPaymentWizardDraft(propertyId, 'payment');
  clearPaymentWizardDraft(propertyId, 'autopayment');
}

export function usePaymentWizardDraft(
  propertyId: string,
  type: PaymentDraftType,
): {
  readonly draft: PaymentWizardDraft;
  readonly isLoaded: boolean;
  /** Есть ли что продолжать: хоть одно заполненное поле шага. */
  readonly hasDraft: boolean;
  readonly setDraft: Dispatch<SetStateAction<PaymentWizardDraft>>;
  readonly clearDraft: () => void;
} {
  const { draft, isLoaded, setDraft, clearDraft } = useDraftStore<PaymentWizardDraft>({
    storageKey: paymentDraftStorageKey(propertyId, type),
    createDefault: () => DEFAULT_DRAFT,
    validate: validatePaymentWizardDraft,
    storage: 'local',
  });
  // Каждая правка визарда двигает свежесть черновика: время живёт в самом
  // payload, поэтому «последний тронутый» переживает перезагрузку страницы.
  const setDraftStamped: Dispatch<SetStateAction<PaymentWizardDraft>> = (value) => {
    setDraft((prev) => ({
      ...(typeof value === 'function' ? value(prev) : value),
      updatedAt: Date.now(),
    }));
  };
  return {
    draft,
    isLoaded,
    hasDraft: hasPaymentWizardDraftFields(draft),
    setDraft: setDraftStamped,
    clearDraft,
  };
}

/** Черновик наличествует, когда валидатор оставил хоть одно поле шага
 * (служебный updatedAt не в счёт). */
export function hasPaymentWizardDraftFields(draft: PaymentWizardDraft): boolean {
  return (
    draft.type !== undefined
    || draft.categorySlug !== undefined
    || draft.title !== undefined
    || draft.recurrence !== undefined
    || draft.endDate !== undefined
    || draft.amountKopecks !== undefined
    || draft.paymentForm !== undefined
  );
}

/** Тип для шита «Добавить»: последний тронутый черновик объекта; их нет —
 * undefined. При равной свежести (нет таймстампов у унаследованных
 * черновиков) — обычный платёж, детерминированно. */
export function latestPaymentDraftType(
  payment: { readonly hasDraft: boolean; readonly draft: PaymentWizardDraft },
  autopayment: { readonly hasDraft: boolean; readonly draft: PaymentWizardDraft },
): PaymentDraftType | undefined {
  if (payment.hasDraft && autopayment.hasDraft) {
    return (autopayment.draft.updatedAt ?? 0) > (payment.draft.updatedAt ?? 0)
      ? 'autopayment'
      : 'payment';
  }
  if (payment.hasDraft) return 'payment';
  if (autopayment.hasDraft) return 'autopayment';
  return undefined;
}

function isFilledString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

function isPositiveInt(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0;
}

function validateRecurrence(value: unknown): Recurrence | undefined {
  if (value === null || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  switch (record.kind) {
    case 'daily':
      return { kind: 'daily' };
    case 'weekly':
      return Array.isArray(record.weekdays) && record.weekdays.every((day) => typeof day === 'number')
        ? { kind: 'weekly', weekdays: record.weekdays }
        : undefined;
    case 'monthly':
      return isPositiveInt(record.dayOfMonth) ? { kind: 'monthly', dayOfMonth: record.dayOfMonth } : undefined;
    case 'yearly':
      return isPositiveInt(record.month) && isPositiveInt(record.day)
        ? { kind: 'yearly', month: record.month, day: record.day }
        : undefined;
    default:
      return undefined;
  }
}

/** Форма-проверка persisted payload: неизвестные значения роняют весь
 * черновик (как в use-property-create-draft), пустые строки и неположительные
 * суммы отбрасываются — мусор из хранилища не всплывает в визарде. */
export function validatePaymentWizardDraft(parsed: unknown): PaymentWizardDraft {
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return DEFAULT_DRAFT;

  const record = parsed as Record<string, unknown>;

  const type = record.type === 'income' || record.type === 'expense' ? record.type : undefined;
  if (record.type !== undefined && type === undefined) return DEFAULT_DRAFT;

  const categorySlug = isFilledString(record.categorySlug) ? record.categorySlug : undefined;
  if (record.categorySlug !== undefined && categorySlug === undefined) return DEFAULT_DRAFT;

  const title = isFilledString(record.title) ? record.title : undefined;

  const recurrence =
    record.recurrence !== undefined
      ? validateRecurrence(record.recurrence)
      : undefined;
  if (record.recurrence !== undefined && recurrence === undefined) return DEFAULT_DRAFT;

  const endDate = typeof record.endDate === 'string' && record.endDate.length > 0 ? record.endDate : undefined;

  const amountKopecks = isPositiveInt(record.amountKopecks) ? record.amountKopecks : undefined;
  if (record.amountKopecks !== undefined && amountKopecks === undefined) return DEFAULT_DRAFT;

  const paymentForm =
    record.paymentForm === 'transfer' || record.paymentForm === 'cash' ? record.paymentForm : undefined;
  if (record.paymentForm !== undefined && paymentForm === undefined) return DEFAULT_DRAFT;

  // Служебное поле: мусорный таймстамп просто отбрасывается, черновик
  // не роняет.
  const updatedAt = isPositiveInt(record.updatedAt) ? record.updatedAt : undefined;

  return {
    ...(type !== undefined && { type }),
    ...(categorySlug !== undefined && { categorySlug }),
    ...(title !== undefined && { title }),
    ...(recurrence !== undefined && { recurrence }),
    ...(endDate !== undefined && { endDate }),
    ...(amountKopecks !== undefined && { amountKopecks }),
    ...(paymentForm !== undefined && { paymentForm }),
    ...(updatedAt !== undefined && { updatedAt }),
  };
}
