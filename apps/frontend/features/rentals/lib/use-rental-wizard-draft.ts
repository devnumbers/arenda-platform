'use client';

import type { Dispatch, SetStateAction } from 'react';
import { useDraftStore } from '@/shared/lib/hooks/useDraftStore';
import type { IsoDate } from '@/shared/lib/calendar';
import type { RentalPaymentDay, RentalUtilities } from '@/entities/rental';

/**
 * Черновик визарда создания аренды (#530): переживает перезагрузку страницы
 * и уход в ветку создания контакта (#509) — хранится в localStorage
 * (`storage: 'local'`), ключ per объект. Обёртка владеет только ключом,
 * дефолтом и проверкой формы — сам цикл хранения живёт в useDraftStore.
 * Черновик покрывает поля всех четырёх шагов; готовность шагов —
 * wizard-model, шаги UI — слой экрана.
 */
export type RentalWizardDraft = {
  /** Арендная плата в копейках, целая положительная (шаг 1). */
  readonly amountKopecks?: number;
  /** День оплаты: число месяца 1–31 или «последний день» (шаг 1). */
  readonly paymentDay?: RentalPaymentDay;
  /** Автоплатёж Платежа арендной платы (шаг 2; отсутствие = выключен). */
  readonly autoPay?: boolean;
  /** Начало аренды — сегодня или позже (шаг 3). */
  readonly startDate?: IsoDate;
  /** Плановое окончание; отсутствие — бессрочная аренда (шаг 3). */
  readonly plannedEndDate?: IsoDate;
  /** Коммунальные платежи (шаг 3). */
  readonly utilities?: RentalUtilities;
  /** Залог в копейках (шаг 3; отсутствие — не задан). */
  readonly depositKopecks?: number;
  /** Комиссия в копейках (шаг 3; отсутствие — не задана). */
  readonly commissionKopecks?: number;
  /** Арендатор — контакт из книги объекта (шаг 4; отсутствие — не выбран). */
  readonly contactId?: string;
};

const DEFAULT_DRAFT: RentalWizardDraft = {};

export function rentalWizardDraftStorageKey(propertyId: string): string {
  return `rental-wizard-draft:${propertyId}`;
}

export function useRentalWizardDraft(propertyId: string): {
  readonly draft: RentalWizardDraft;
  readonly isLoaded: boolean;
  readonly setDraft: Dispatch<SetStateAction<RentalWizardDraft>>;
  readonly clearDraft: () => void;
} {
  return useDraftStore<RentalWizardDraft>({
    storageKey: rentalWizardDraftStorageKey(propertyId),
    createDefault: () => DEFAULT_DRAFT,
    validate: validateRentalWizardDraft,
    storage: 'local',
  });
}

function isIsoDate(value: unknown): value is IsoDate {
  return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value);
}

function isNonNegativeInt(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}

function isFilledString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0;
}

function validatePaymentDay(value: unknown): RentalPaymentDay | undefined {
  if (value === 'last') return 'last';
  return typeof value === 'number' && Number.isInteger(value) && value >= 1 && value <= 31
    ? value
    : undefined;
}

function validateUtilities(value: unknown): RentalUtilities | undefined {
  return value === 'included' || value === 'meters_only' || value === 'full_receipt'
    ? value
    : undefined;
}

/** Форма-проверка persisted payload: неизвестные значения роняют поле
 * (мусор из хранилища не всплывает в визарде), остальные сохраняются. */
export function validateRentalWizardDraft(parsed: unknown): RentalWizardDraft {
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return DEFAULT_DRAFT;
  }

  const record = parsed as Record<string, unknown>;

  const amountKopecks =
    isNonNegativeInt(record.amountKopecks) && record.amountKopecks > 0
      ? record.amountKopecks
      : undefined;
  const paymentDay =
    record.paymentDay !== undefined ? validatePaymentDay(record.paymentDay) : undefined;
  const autoPay = typeof record.autoPay === 'boolean' ? record.autoPay : undefined;
  const startDate = isIsoDate(record.startDate) ? record.startDate : undefined;
  const plannedEndDate = isIsoDate(record.plannedEndDate) ? record.plannedEndDate : undefined;
  const utilities =
    record.utilities !== undefined ? validateUtilities(record.utilities) : undefined;
  const depositKopecks = isNonNegativeInt(record.depositKopecks)
    ? record.depositKopecks
    : undefined;
  const commissionKopecks = isNonNegativeInt(record.commissionKopecks)
    ? record.commissionKopecks
    : undefined;
  const contactId = isFilledString(record.contactId) ? record.contactId : undefined;

  return {
    ...(amountKopecks !== undefined && { amountKopecks }),
    ...(paymentDay !== undefined && { paymentDay }),
    ...(autoPay !== undefined && { autoPay }),
    ...(startDate !== undefined && { startDate }),
    ...(plannedEndDate !== undefined && { plannedEndDate }),
    ...(utilities !== undefined && { utilities }),
    ...(depositKopecks !== undefined && { depositKopecks }),
    ...(commissionKopecks !== undefined && { commissionKopecks }),
    ...(contactId !== undefined && { contactId }),
  };
}
